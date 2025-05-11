package portfolio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/invest/instrument"
	"github.com/STTM-NSU/trading-bot/internal/invest/position"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
	"github.com/jmoiron/sqlx"
)

const (
	_flushInterval = 1 * time.Hour
)

type Portfolio struct {
	db     *sqlx.DB
	logger logger.Logger

	instrumentsService *instrument.InstrumentsService
	positionsService   *position.PositionsService

	mu sync.RWMutex

	accountID     string
	profitPercent float64
	instruments   map[string]model.PortfolioInstrument
	balance       map[string]float64
}

func NewPortfolio(
	accountID string,
	instrumentsService *instrument.InstrumentsService,
	positionsService *position.PositionsService,
	initBalances []model.Balance,
	db *sqlx.DB,
	logger logger.Logger) *Portfolio {
	balances := make(map[string]float64)
	for _, balance := range initBalances {
		balances[balance.Currency] = balance.Value
	}

	return &Portfolio{
		db:                 db,
		instrumentsService: instrumentsService,
		positionsService:   positionsService,
		logger:             logger,
		accountID:          accountID,
		balance:            balances,
		instruments:        make(map[string]model.PortfolioInstrument),
	}
}

func (p *Portfolio) Init(ctx context.Context) error {
	exists, err := p.LoadFromDB(ctx)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	investBalances, err := p.positionsService.UnaryGetPositions()
	if err != nil {
		return err
	}
	for _, investBalance := range investBalances {
		if p.balance[investBalance.Currency] > investBalance.Value {
			return fmt.Errorf("invest balance %f less than initial balance %f", investBalance.Value, p.balance[investBalance.Currency])
		}
	}

	return nil
}

func (p *Portfolio) GetBalance(curr string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.balance[curr]
}

func (p *Portfolio) UpdateBalance(diff model.Balance) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.balance[diff.Currency] += diff.Value
}

func (p *Portfolio) GetInstruments() ([]model.PortfolioInstrument, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	instruments := make([]model.PortfolioInstrument, 0, len(p.instruments))
	for _, i := range p.instruments {
		instruments = append(instruments, i)
	}
	return instruments, nil
}

func (p *Portfolio) UpdateInstrument(i model.PortfolioInstrument) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.instruments[i.InstrumentID] = i
}

func (p *Portfolio) RemoveInstrument(i model.PortfolioInstrument) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.instruments, i.InstrumentID)
}

func (p *Portfolio) GetProfit() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.profitPercent
}

func (p *Portfolio) UpdateProfit(profit float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.profitPercent += profit
}

func (p *Portfolio) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(_flushInterval):
			if err := p.FlushToDB(ctx); err != nil {
				p.logger.Errorf("%s: error flushing portfolio", err)
			}
		}
	}
}
