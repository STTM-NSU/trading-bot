package portfolio

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/STTM-NSU/trading-bot/internal/model"
)

const (
	_queryPortfolio            = "SELECT account_id, profit_percent FROM portfolios WHERE account_id = $1"
	_queryBalance              = "SELECT value, currency FROM balances WHERE account_id = $1"
	_queryPortfolioInstruments = "SELECT * FROM portfolio_instruments WHERE account_id = $1"
)

func (p *Portfolio) LoadFromDB(ctx context.Context) (bool, error) {
	var (
		portf       model.Portfolio
		balances    []model.Balance
		instruments []model.PortfolioInstrument
		exists      bool
	)
	if err := p.db.GetContext(ctx, &portf, _queryPortfolio, p.accountID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return exists, nil
		}
		return exists, fmt.Errorf("%w: can't query portfolio", err)
	}
	exists = true

	if err := p.db.SelectContext(ctx, balances, _queryBalance, p.accountID); err != nil {
		return exists, fmt.Errorf("%w: can't query portfolio balances", err)
	}

	if err := p.db.SelectContext(ctx, instruments, _queryPortfolioInstruments, p.accountID); err != nil {
		return exists, fmt.Errorf("%w: can't query portfolio instruments", err)
	}

	p.balance = make(map[string]float64)
	for _, balance := range balances {
		p.balance[balance.Currency] = balance.Value
	}

	p.instruments = make(map[string]model.PortfolioInstrument)
	for _, instrument := range instruments {
		p.instruments[instrument.InstrumentID] = instrument
	}

	p.profitPercent = portf.ProfitPercent
	return exists, nil
}

const (
	_updatePortfolio   = "UPDATE portfolios SET profit_percent = $1 WHERE account_id = $2"
	_updateInstruments = `INSERT INTO portfolio_instruments (
								instrument_id,
								order_request_id,
                                order_id,
								direction,
							    instrument_type,
								entry_price,
								quantity,
							   	min_price_increment,
								account_id
							) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
							ON CONFLICT (instrument_id) 
							DO UPDATE SET
								order_request_id = EXCLUDED.order_request_id,
								order_id = EXCLUDED.order_id,
								direction = EXCLUDED.direction,
								instrument_type = EXCLUDED.instrument_type,
								entry_price = EXCLUDED.entry_price,
								quantity = EXCLUDED.quantity,
								min_price_increment = EXCLUDED.min_price_increment,
								account_id = EXCLUDED.account_id;`
	_updateBalance = `INSERT INTO balances (
								value, currency, account_id
							) VALUES ($1,$2,$3)
							ON CONFLICT ON CONSTRAINT currency_account_id
							DO UPDATE SET
								value = EXCLUDED.value,
								currency = EXCLUDED.currency;`
)

func (p *Portfolio) FlushToDB(ctx context.Context) error {
	if len(p.instruments) == 0 || len(p.balance) == 0 {
		return nil
	}

	if _, err := p.db.ExecContext(ctx, _updatePortfolio, p.profitPercent, p.accountID); err != nil {
		return fmt.Errorf("%w: can't update portfolio", err)
	}
	for _, instrument := range p.instruments {
		if _, err := p.db.ExecContext(ctx, _updateInstruments,
			instrument.InstrumentID,
			instrument.OrderRequestID,
			instrument.OrderID,
			instrument.Direction,
			instrument.InstrumentType,
			instrument.EntryPrice,
			instrument.Quantity,
			instrument.MinPriceIncrement,
			cmp.Or(instrument.AccountID, p.accountID),
		); err != nil {
			return fmt.Errorf("%w: can't update portfolio instruments", err)
		}
	}

	for curr, value := range p.balance {
		if _, err := p.db.ExecContext(ctx, _updateBalance, value, curr, p.accountID); err != nil {
			return fmt.Errorf("%w: can't update portfolio balance", err)
		}
	}

	return nil
}
