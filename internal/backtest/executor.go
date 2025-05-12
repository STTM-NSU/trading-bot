package backtest

import (
	"sync"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/invest/md"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
)

type Direction int

const (
	Buy Direction = iota + 1
	Sell
)

type TrackingInstrument struct {
	instrumentId   string // uid
	figi           string
	quantity       float64
	lot            float64
	instrumentType model.InstrumentType

	i           model.PortfolioInstrument
	hedgePrice  float64
	wantedPrice float64
	market      bool
	direction   Direction
}

type Executor struct {
	logger logger.Logger

	mu          sync.Mutex
	instruments map[string]TrackingInstrument
	taxes       map[model.InstrumentType]float64

	candlesService *md.CandlesService
	portfolio      *Portfolio
}

func NewExecutor(logger logger.Logger, taxes map[model.InstrumentType]float64, candlesService *md.CandlesService, portfolio *Portfolio) *Executor {
	return &Executor{
		logger:         logger,
		taxes:          taxes,
		portfolio:      portfolio,
		candlesService: candlesService,
		instruments:    make(map[string]TrackingInstrument),
	}
}

func (e *Executor) Check(from time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Debugf("checking trading instruments %d", len(e.instruments))

	for _, instr := range e.instruments {
		if instr.direction == Sell {
			price, err := e.candlesService.GetLastPriceOn(instr.figi, from)
			if err != nil {
				e.logger.Errorf("GetLastPriceOn exec check err: %v", err)
				continue
			}
			instrPrice := instr.quantity * instr.lot * price * (1 - e.taxes[instr.instrumentType])
			if instr.market {
				e.logger.Infof("sell market %s %f %f %f %f", instr.instrumentId, instrPrice, instr.quantity, instr.lot, price)
				e.portfolio.UpdateBalance(instrPrice, instr.instrumentId)
				e.portfolio.RemoveInstrument(instr.instrumentId)
				delete(e.instruments, instr.instrumentId)
			} else if instr.wantedPrice <= instrPrice || instr.hedgePrice > instrPrice {
				e.logger.Infof("sell limit %s [%f, %f] %f %f %f %f", instr.instrumentId, instr.wantedPrice, instr.hedgePrice,
					instrPrice, instr.quantity, instr.lot, price)
				e.portfolio.UpdateBalance(instrPrice, instr.instrumentId)
				e.portfolio.RemoveInstrument(instr.instrumentId)
				delete(e.instruments, instr.instrumentId)
			}
		}
	}

	for _, instr := range e.instruments {
		if instr.direction == Buy && instr.market {
			price, err := e.candlesService.GetLastPriceOn(instr.figi, from)
			if err != nil {
				e.logger.Errorf("GetLastPriceOn exec err: %v", err)
				continue
			}
			instrPrice := instr.quantity * instr.lot * price * (1 + e.taxes[instr.instrumentType])
			if e.portfolio.GetBalance() >= instrPrice {
				e.logger.Infof("buy %s %f %f %f %f", instr.instrumentId, instrPrice, instr.quantity, instr.lot, price)
				e.portfolio.Buy(instrPrice)
				e.portfolio.AddInstrument(model.PortfolioInstrument{
					FIGI:           instr.figi,
					InstrumentType: string(instr.instrumentType),
					EntryPrice:     instrPrice,
					Quantity:       instr.quantity,
					Lot:            instr.lot,
					InstrumentID:   instr.instrumentId,
				})
				delete(e.instruments, instr.instrumentId)
			}
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
			e.instruments[id] = TrackingInstrument{
				instrumentId:   v.instrumentId,
				figi:           v.figi,
				lot:            v.lot,
				quantity:       v.quantity + instr.Quantity,
				instrumentType: v.instrumentType,
				i:              v.i,
				market:         true,
				direction:      Sell,
			}

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

func (e *Executor) SellLimit(price float64, profit, hedge float64, i model.PortfolioInstrument) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.instruments[i.InstrumentID]; ok {
		e.logger.Errorf("SellLimit: instrument %v already exists", i.InstrumentID)
		return
	}

	p := i.EntryPrice
	if price*i.Lot*i.Quantity > i.EntryPrice {
		p = price * i.Lot * i.Quantity
	}

	e.instruments[i.InstrumentID] = TrackingInstrument{
		instrumentId:   i.InstrumentID,
		figi:           i.FIGI,
		quantity:       i.Quantity,
		lot:            i.Lot,
		instrumentType: model.InstrumentType(i.InstrumentType),
		i:              i,
		hedgePrice:     p * (1 - hedge),
		wantedPrice:    p * (1 + profit),
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

	if v, ok := e.instruments[i.UID]; ok && v.direction == Buy {
		e.instruments[i.UID] = TrackingInstrument{
			figi:           v.figi,
			instrumentId:   v.instrumentId,
			lot:            v.lot,
			quantity:       v.quantity + q,
			instrumentType: v.instrumentType,
			market:         v.market,
			direction:      v.direction,
		}
		return
	}
	e.instruments[i.UID] = TrackingInstrument{
		figi:           i.FIGI,
		instrumentId:   i.UID,
		lot:            float64(i.Lot),
		quantity:       q,
		instrumentType: i.InstrumentType,
		market:         true,
		direction:      Buy,
	}
}
