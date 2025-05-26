package backtest

import (
	"sync"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/invest/md"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
)

type Portfolio struct {
	logger logger.Logger

	mu           sync.Mutex
	balance      float64
	entryBalance float64

	instruments    map[string]model.PortfolioInstrument
	candlesService *md.CandlesService
}

func NewPortfolio(logger logger.Logger, balance float64, cs *md.CandlesService) *Portfolio {
	return &Portfolio{
		logger:         logger,
		balance:        balance,
		entryBalance:   balance,
		candlesService: cs,
		instruments:    make(map[string]model.PortfolioInstrument),
	}
}

func (p *Portfolio) GetInstrument(id string) model.PortfolioInstrument {
	p.mu.Lock()
	defer p.mu.Unlock()

	if v, ok := p.instruments[id]; ok {
		return v
	}

	return model.PortfolioInstrument{}
}

func (p *Portfolio) AddInstrument(i model.PortfolioInstrument) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.instruments[i.InstrumentID]; ok {
		return
	}

	p.instruments[i.InstrumentID] = i
}

func (p *Portfolio) RemoveInstrument(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.instruments, id)
}

func (p *Portfolio) GetInstruments() map[string]model.PortfolioInstrument {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.instruments
}

func (p *Portfolio) GetBalanceWithInstruments(from time.Time) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	sum := p.balance
	for _, v := range p.instruments {
		// lp, err := p.candlesService.GetLastPriceOnDB(v.FIGI, from)
		// if err != nil {
		sum += v.EntryPrice
		// 	continue
		// }
		// sum += v.Lot * v.Quantity * lp
	}

	return sum
}

func (p *Portfolio) GetBalance() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.balance
}

func (p *Portfolio) GetProfit(from time.Time) float64 {
	balance := p.GetBalanceWithInstruments(from)

	p.mu.Lock()
	defer p.mu.Unlock()
	return (balance - p.entryBalance) / p.entryBalance * 100
}

func (p *Portfolio) UpdateBalance(sellPrice float64, id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.balance += sellPrice

	if v, ok := p.instruments[id]; ok {
		p.logger.Infof("sell with price %f, profit %f percent", sellPrice, (sellPrice-v.EntryPrice)/sellPrice*100)
	} else {
		p.logger.Warnf("sell with price %f unknown id: %s", sellPrice, id)
	}
}

func (p *Portfolio) UpdateBalanceMargin(buyPrice float64, entryPrice float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	profit := entryPrice - buyPrice
	p.balance += profit
	p.logger.Infof("margin with price %f, profit %f percent", profit, profit/buyPrice*100)
}

func (p *Portfolio) Buy(price float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.balance -= price
}
