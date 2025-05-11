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
			price, err := e.candlesService.GetLastPriceOn(instr.instrumentId, from)
			if err != nil {
				e.logger.Errorf("GetLastPriceOn exec check err: %v", err)
				continue
			}
			if instr.market {
				instrPrice := instr.quantity * instr.lot * price * (1 - e.taxes[instr.instrumentType])
				e.logger.Infof("sell market %s %f %f %f %f", instr.instrumentId, instrPrice, instr.quantity, instr.lot, price)
				e.portfolio.UpdateBalance(instrPrice, instr.instrumentId)
				e.portfolio.RemoveInstrument(instr.instrumentId)
				delete(e.instruments, instr.instrumentId)
			} else if instr.wantedPrice <= price || instr.hedgePrice > price {
				instrPrice := instr.quantity * instr.lot * price * (1 - e.taxes[instr.instrumentType])
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
			price, err := e.candlesService.GetLastPriceOn(instr.instrumentId, from)
			if err != nil {
				e.logger.Errorf("GetLastPriceOn exec err: %v", err)
				continue
			}
			instrPrice := instr.quantity * instr.lot * price * (1 + e.taxes[instr.instrumentType])
			if e.portfolio.GetBalance() >= instrPrice {
				e.logger.Infof("buy %s %f %f %f %f", instr.instrumentId, instrPrice, instr.quantity, instr.lot, price)
				e.portfolio.Buy(instrPrice)
				e.portfolio.AddInstrument(model.PortfolioInstrument{
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

// SellOut on rebalance if limit orders were not executed
func (e *Executor) SellOut() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for id, instr := range e.instruments {
		if instr.direction == Sell {
			delete(e.instruments, id)
			e.SellMarket(model.PortfolioInstrument{
				InstrumentType: string(instr.instrumentType),
				InstrumentID:   instr.instrumentId,
				Quantity:       instr.quantity,
				Lot:            instr.lot,
			})
		}
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
	if price > i.EntryPrice {
		p = price
	}

	e.instruments[i.InstrumentID] = TrackingInstrument{
		instrumentId:   i.InstrumentID,
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
		instrumentId:   i.UID,
		lot:            float64(i.Lot),
		quantity:       q,
		instrumentType: i.InstrumentType,
		market:         true,
		direction:      Buy,
	}
}
