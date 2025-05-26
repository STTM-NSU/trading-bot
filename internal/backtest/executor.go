package backtest

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/invest/md"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
)

type Direction int

const (
	Buy Direction = iota + 1
	Sell
	NewShort // without price
	Short    // owned
)

type TrackingInstrument struct {
	instrumentId   string // uid
	figi           string
	quantity       float64
	lot            float64
	instrumentType model.InstrumentType

	i             model.PortfolioInstrument
	origPrice     float64
	hedgePercent  float64
	profitPercent float64
	market        bool
	direction     Direction

	marginTaxDay time.Time
}

type IntervalProfit struct {
	Balance float64
	Profit  float64
	Ts      time.Time
}

type Executor struct {
	logger logger.Logger

	mu          sync.Mutex
	instruments map[string]TrackingInstrument
	taxes       map[model.InstrumentType]float64
	marginTaxes map[float64]float64

	candlesService *md.CandlesService
	portfolio      *Portfolio
	ordersCfg      config.OrdersConfig

	info    []IntervalProfit
	lastDay time.Time
}

func NewExecutor(
	logger logger.Logger,
	taxes map[model.InstrumentType]float64, marginTaxes map[float64]float64,
	candlesService *md.CandlesService, portfolio *Portfolio, ordersCfg config.OrdersConfig) *Executor {
	return &Executor{
		logger:         logger,
		taxes:          taxes,
		marginTaxes:    marginTaxes,
		portfolio:      portfolio,
		candlesService: candlesService,
		ordersCfg:      ordersCfg,
		instruments:    make(map[string]TrackingInstrument),
		info:           make([]IntervalProfit, 0),
	}
}

func (e *Executor) GetInfo() []IntervalProfit {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.info
}

func getMarginTaxes(price float64, taxes map[float64]float64) float64 {
	for _, p := range slices.Sorted(maps.Keys(taxes)) {
		if price < p {
			return taxes[p]
		}
	}
	return 0
}

func (e *Executor) Check(from time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Debugf("checking trading instruments %d", len(e.instruments))

	// Sell stage
	for _, instr := range e.instruments {
		if instr.direction == Sell {
			e.checkSell(instr, from)
		}
	}

	// Margin stage
	for _, instr := range e.instruments {
		switch instr.direction {
		case NewShort: // sell
			e.checkNewShort(instr, from)
		case Short: // buy
			e.checkShort(instr, from)
		default:
		}
	}

	e.updateInfo(from)

	// Buy stage
	for _, instr := range e.instruments {
		if instr.direction == Buy {
			e.checkBuy(instr, from)
		}
	}
}

func (e *Executor) CheckTogether(from time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, instr := range e.instruments {
		switch instr.direction {
		case Sell:
			e.checkSell(instr, from)
		case NewShort:
			e.checkNewShort(instr, from)
		case Short:
			e.checkShort(instr, from)
		case Buy:
			e.checkBuy(instr, from)
		}
	}

	e.updateInfo(from)
}

func (e *Executor) checkSell(instr TrackingInstrument, from time.Time) {
	price, err := e.candlesService.GetLastPriceOn(instr.figi, from)
	if err != nil {
		// e.logger.Errorf("GetLastPriceOn exec check err: %v", err)
		return
	}
	instrPrice := instr.quantity * instr.lot * price * (1 - e.taxes[instr.instrumentType])
	if instr.market {
		e.logger.Infof("sell market %s %f %f %f %f", instr.instrumentId, instrPrice, instr.quantity, instr.lot, price)
		e.portfolio.UpdateBalance(instrPrice, instr.instrumentId)
		e.portfolio.RemoveInstrument(instr.instrumentId)
		delete(e.instruments, instr.instrumentId)
	} else if instr.profitPercent*instr.origPrice <= instrPrice ||
		instr.hedgePercent*instr.origPrice > instrPrice {
		e.logger.Infof("sell limit %s [%f, %f] %f %f %f %f", instr.instrumentId,
			instr.profitPercent*instr.origPrice, instr.hedgePercent*instr.origPrice,
			instrPrice, instr.quantity, instr.lot, price)
		e.portfolio.UpdateBalance(instrPrice, instr.instrumentId)
		e.portfolio.RemoveInstrument(instr.instrumentId)
		delete(e.instruments, instr.instrumentId)
	}
}

func (e *Executor) checkNewShort(instr TrackingInstrument, from time.Time) {
	price, err := e.candlesService.GetLastPriceOn(instr.figi, from)
	if err != nil {
		// e.logger.Errorf("GetLastPriceOn exec check err: %v", err)
		return
	}
	instrPrice := instr.quantity * instr.lot * price * (1 - e.taxes[instr.instrumentType])
	instr.origPrice = instrPrice
	instr.marginTaxDay = from.Truncate(24 * time.Hour)
	instr.direction = Short
	e.instruments[instr.instrumentId] = instr
	e.logger.Infof("open short %s [%f %f>%f] %f %f %f", instr.instrumentId,
		instrPrice, instr.profitPercent*instr.origPrice, instr.hedgePercent*instr.origPrice,
		instr.quantity, instr.lot, price)
}

func (e *Executor) checkShort(instr TrackingInstrument, from time.Time) {
	if instr.marginTaxDay != from.Truncate(24*time.Hour) {
		instr.marginTaxDay = from.Truncate(24 * time.Hour)
		tax := getMarginTaxes(instr.origPrice, e.marginTaxes)
		e.logger.Infof("taxes for short %s [%f, %f]", instr.instrumentId, instr.origPrice, tax)
		instr.origPrice -= tax
		e.instruments[instr.instrumentId] = instr
	}

	price, err := e.candlesService.GetLastPriceOn(instr.figi, from)
	if err != nil {
		// e.logger.Errorf("GetLastPriceOn exec check err: %v", err)
		return
	}
	instrPrice := instr.quantity * instr.lot * price * (1 + e.taxes[instr.instrumentType])
	if instr.market { // buy out
		e.logger.Infof("close short market %s [%f > %f > %f] [%f] %f %f %f",
			instr.instrumentId,
			instr.origPrice*instr.profitPercent, instr.origPrice, instr.origPrice*instr.hedgePercent,
			instrPrice, instr.quantity, instr.lot, price)
		e.portfolio.UpdateBalanceMargin(instrPrice, instr.origPrice)
		delete(e.instruments, instr.instrumentId)
	} else if instrPrice <= instr.origPrice*instr.profitPercent { // profit
		e.logger.Infof("close short profit %s [%f > %f > %f] [%f] %f %f %f",
			instr.instrumentId,
			instr.origPrice*instr.profitPercent, instr.origPrice, instr.origPrice*instr.hedgePercent,
			instrPrice, instr.quantity, instr.lot, price)
		e.portfolio.UpdateBalanceMargin(instrPrice, instr.origPrice)
		delete(e.instruments, instr.instrumentId)
	} else if instrPrice > instr.origPrice*instr.hedgePercent {
		e.logger.Infof("close short hedge %s [%f > %f > %f] [%f] %f %f %f",
			instr.instrumentId,
			instr.origPrice*instr.profitPercent, instr.origPrice, instr.origPrice*instr.hedgePercent,
			instrPrice, instr.quantity, instr.lot, price)
		e.portfolio.UpdateBalanceMargin(instrPrice, instr.origPrice)
		delete(e.instruments, instr.instrumentId)
	}
}

func (e *Executor) checkBuy(instr TrackingInstrument, from time.Time) {
	if !instr.market {
		return
	}

	price, err := e.candlesService.GetLastPriceOn(instr.figi, from)
	if err != nil {
		// e.logger.Errorf("GetLastPriceOn exec err: %v", err)
		return
	}
	instrPrice := instr.quantity * instr.lot * price * (1 + e.taxes[instr.instrumentType])
	if e.portfolio.GetBalance() >= instrPrice {
		e.logger.Infof("buy %s %f %f %f %f", instr.instrumentId, instrPrice, instr.quantity, instr.lot, price)
		e.portfolio.Buy(instrPrice)
		portfolioInstr := model.PortfolioInstrument{
			FIGI:           instr.figi,
			InstrumentType: string(instr.instrumentType),
			EntryPrice:     instrPrice,
			Quantity:       instr.quantity,
			Lot:            instr.lot,
			InstrumentID:   instr.instrumentId,
		}
		e.portfolio.AddInstrument(portfolioInstr)
		delete(e.instruments, instr.instrumentId)

		e.instruments[instr.instrumentId] = TrackingInstrument{
			instrumentId:   instr.instrumentId,
			figi:           instr.figi,
			quantity:       instr.quantity,
			lot:            instr.lot,
			instrumentType: instr.instrumentType,
			i:              portfolioInstr,
			origPrice:      instrPrice,
			hedgePercent:   1 - e.ordersCfg.SellOrder.DefencePercentIndent,
			profitPercent:  1 + e.ordersCfg.SellOrder.ProfitPercentIndent,
			direction:      Sell,
		}
	}
}

func (e *Executor) updateInfo(from time.Time) {
	fromDay := from.Truncate(12 * time.Hour)
	if e.lastDay != fromDay && fromDay.Hour() != 0 {
		balance := e.portfolio.GetBalanceWithInstruments(from)
		e.logger.Infof("Portfolio balance: %f on %s", balance, from)
		e.lastDay = fromDay
		e.info = append(e.info, IntervalProfit{
			Balance: balance,
			Profit:  e.portfolio.GetProfit(from),
			Ts:      fromDay,
		})
	}
}

func (e *Executor) BuyDeptMargin() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, instr := range e.instruments {
		switch instr.direction {
		case NewShort:
			delete(e.instruments, instr.instrumentId)
		case Short:
			instr.market = true
			e.instruments[instr.instrumentId] = instr
		default:
		}
	}
}

// RemoveBuyOrders in case there are remaining buy orders on next rebalance
func (e *Executor) RemoveBuyOrders() {
	e.mu.Lock()
	defer e.mu.Unlock()

	instrs := make(map[string]TrackingInstrument, len(e.instruments))
	for id, instr := range e.instruments {
		if instr.direction != Buy {
			instrs[id] = instr
		}
	}
	e.instruments = instrs
}

func (e *Executor) SellOutPortfolio() {
	e.mu.Lock()
	defer e.mu.Unlock()

	portfolio := e.portfolio.GetInstruments()

	toAdd := make([]TrackingInstrument, 0, len(e.instruments))

	for id, instr := range portfolio {
		if v, ok := e.instruments[id]; ok && v.direction == Sell {
			toAdd = append(toAdd, TrackingInstrument{
				instrumentId:   v.instrumentId,
				figi:           v.figi,
				lot:            v.lot,
				quantity:       v.quantity,
				instrumentType: v.instrumentType,
				i:              v.i,
				market:         true,
				direction:      Sell,
			})
			delete(e.instruments, id)
			continue
		}
		toAdd = append(toAdd, TrackingInstrument{
			instrumentId:   instr.InstrumentID,
			figi:           instr.FIGI,
			lot:            instr.Lot,
			quantity:       instr.Quantity,
			instrumentType: model.InstrumentType(instr.InstrumentType),
			i:              instr,
			market:         true,
			direction:      Sell,
		})
	}

	e.logger.Infof("sell out portfolio %v", toAdd)

	for _, instr := range toAdd {
		e.instruments[instr.instrumentId] = instr
	}
}

// SellOut on rebalance if limit orders were not executed
func (e *Executor) SellOut() {
	e.mu.Lock()
	defer e.mu.Unlock()

	toAdd := make([]TrackingInstrument, 0, len(e.instruments))

	for id, instr := range e.instruments {
		if instr.direction == Sell {
			delete(e.instruments, id)
			toAdd = append(toAdd, TrackingInstrument{
				instrumentId:   instr.instrumentId,
				figi:           instr.figi,
				lot:            instr.lot,
				quantity:       instr.quantity,
				instrumentType: instr.instrumentType,
				i: model.PortfolioInstrument{
					FIGI:           instr.figi,
					InstrumentType: string(instr.instrumentType),
					InstrumentID:   instr.instrumentId,
					Quantity:       instr.quantity,
					Lot:            instr.lot,
				},
				market:    true,
				direction: Sell,
			})
		}
	}

	e.logger.Infof("sell out %v", toAdd)

	for _, instr := range toAdd {
		e.instruments[instr.instrumentId] = instr
	}
}

func (e *Executor) changeCoefficients(profit, hedge float64, i model.PortfolioInstrument, dir Direction) {
	v, ok := e.instruments[i.InstrumentID]
	if !ok {
		return
	}
	if dir != v.direction {
		e.logger.Warnf("diff dirs for %s %d %d", i.InstrumentID, dir, v.direction)
		return
	}
	switch v.direction {
	case Sell:
		v.hedgePercent = 1 - hedge
		v.profitPercent = 1 + profit
	case NewShort, Short:
		v.hedgePercent = 1 + hedge
		v.profitPercent = 1 - profit
	default:
		return
	}
	e.instruments[i.InstrumentID] = v
}

func (e *Executor) SellLimit(profit, hedge float64, i model.PortfolioInstrument) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.instruments[i.InstrumentID]; ok {
		e.changeCoefficients(profit, hedge, i, Sell)
		e.logger.Infof("change sell coeffs for %s %f %f", i.InstrumentID, profit, hedge)
		return
	}

	e.instruments[i.InstrumentID] = TrackingInstrument{
		instrumentId:   i.InstrumentID,
		figi:           i.FIGI,
		quantity:       i.Quantity,
		lot:            i.Lot,
		instrumentType: model.InstrumentType(i.InstrumentType),
		i:              i,
		origPrice:      i.EntryPrice,
		hedgePercent:   1 - hedge,
		profitPercent:  1 + profit,
		direction:      Sell,
	}
}

func (e *Executor) SellMarket(i model.PortfolioInstrument) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.instruments[i.InstrumentID]; ok {
		e.logger.Errorf("SellMarket: instrument %v already exists", i.InstrumentID)
		return
	}
	e.instruments[i.InstrumentID] = TrackingInstrument{
		instrumentId:   i.InstrumentID,
		figi:           i.FIGI,
		lot:            i.Lot,
		quantity:       i.Quantity,
		instrumentType: model.InstrumentType(i.InstrumentType),
		i:              i,
		market:         true,
		direction:      Sell,
	}
}

func (e *Executor) BuyMarket(q float64, i model.Instrument) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if v, ok := e.instruments[i.UID]; !ok {
		e.instruments[i.UID] = TrackingInstrument{
			figi:           i.FIGI,
			instrumentId:   i.UID,
			lot:            float64(i.Lot),
			quantity:       q,
			instrumentType: i.InstrumentType,
			market:         true,
			direction:      Buy,
		}
	} else if v.direction == Buy {
		e.instruments[i.UID] = TrackingInstrument{
			figi:           v.figi,
			instrumentId:   v.instrumentId,
			lot:            v.lot,
			quantity:       v.quantity + q,
			instrumentType: v.instrumentType,
			market:         v.market,
			direction:      v.direction,
		}
	}
}

// SellMargin
// 1. Just need to sell on trades start
// 2. In check function update entry price and hold in executor until buy all instrument back to broker
// 3. In moment of sell
func (e *Executor) SellMargin(q, profit, hedge float64, i model.Instrument) {
	if q <= 0 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if v, ok := e.instruments[i.UID]; ok {
		e.changeCoefficients(profit, hedge, v.i, Short)
		e.logger.Infof("change margin coeffs for %s %f %f", i.UID, profit, hedge)
		return
	}

	e.instruments[i.UID] = TrackingInstrument{
		instrumentId:   i.UID,
		figi:           i.FIGI,
		quantity:       q,
		lot:            float64(i.Lot),
		instrumentType: i.InstrumentType,
		profitPercent:  1 - profit,
		hedgePercent:   1 + hedge,
		direction:      NewShort,
	}
}
