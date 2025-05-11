package model

var InvestorTaxes = map[InstrumentType]float64{
	Bond:     0.003,
	Share:    0.003,
	Etf:      0.003,
	Currency: 0.009,
}

var TraderTaxes = map[InstrumentType]float64{
	Bond:     0.0005,
	Share:    0.0005,
	Etf:      0.0005,
	Currency: 0.005,
}

var PremiumTaxes = map[InstrumentType]float64{
	Bond:     0.0004,
	Share:    0.0004,
	Etf:      0.0004,
	Currency: 0.004,
}

var OptionTaxes = []float64{0.5, 0.03} // 0,5 ₽ за опцион, но не менее 3% от его цены
