package model

type Portfolio struct {
	AccountID     string  `db:"account_id"`
	ProfitPercent float64 `db:"profit_percent"`
}

type Balance struct {
	Value     float64 `db:"value"`
	Currency  string  `db:"currency"`
	AccountID string  `db:"account_id"`
}

type PortfolioInstrument struct {
	OrderRequestID    string  `db:"order_request_id"`
	OrderID           string  `db:"order_id"`
	Direction         string  `db:"direction"`
	InstrumentType    string  `db:"instrument_type"`
	EntryPrice        float64 `db:"entry_price"`
	Quantity          float64 `db:"quantity"`
	Lot               float64 `db:"lot"` // TODO: add to DB
	MinPriceIncrement float64 `db:"min_price_increment"`
	InstrumentID      string  `db:"instrument_id"`
	AccountID         string  `db:"account_id"`
}

func (p PortfolioInstrument) GetUID() string {
	return p.InstrumentID
}
