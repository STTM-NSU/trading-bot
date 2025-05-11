package config

import (
	"fmt"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/model"
)

type BacktestConfig struct {
	TradingBotConfig
	Taxes    map[model.InstrumentType]float64
	From, To time.Time
}

func parseTimeNoErr(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

var BacktestCfg = BacktestConfig{
	From:  parseTimeNoErr("2024-01-01T00:00:00Z"),
	To:    parseTimeNoErr("2025-01-01T00:00:00Z"),
	Taxes: model.InvestorTaxes,
	TradingBotConfig: TradingBotConfig{
		StartAmountOfMoney: []model.MoneyValue{
			{
				Currency: "RUB",
				Value:    100000,
			},
		},
		Instruments: Instruments{
			IDs: []string{"BBG000QJW156", "BBG000R607Y3", "BBG000RMWQD4",
				// "BBG004730JJ5", "BBG004730N88", "BBG004730RP0", "BBG004730ZJ9",
				// "BBG004731032", "BBG004731354", "BBG004731489", "BBG0047315D0", "BBG0047315Y7",
				// "BBG00475JZZ6", "BBG00475K2X9", "BBG00475K6C3", "BBG00475KHX6", "BBG004RVFFC0",
				// "BBG004S681B4", "BBG004S681M2", "BBG004S681W1", "BBG004S682Z6", "BBG004S683W7",
				// "BBG004S68473", "BBG004S68507", "BBG004S68598", "BBG004S685M3", "BBG004S68614",
				// "BBG004S686W0", "BBG004S68829",
				// "BBG004S689R0", "BBG004S68B31", "BBG004S68BH6", "BBG004S68FR6", "BBG008F2T3T2",
				// "BBG009GSYN76", "BBG00F9XX7H4", "RU000A106T36", "TCS00A0ZZAC4",
				"TCS00A106YF0", "TCS00Y3XYV94", "TCS80A107UL4"},
		},
		STTM: STTMConfig{
			Address:             "http://192.168.0.24:8000",
			TopSTTMPercent:      0.5,
			TopSTTMThreshold:    -1000,
			CalculationInterval: Week,
			STTMHyperparameters: STTMHyperparameters{
				Alpha:     0.05,
				PValue:    0.05,
				Threshold: 0.3,
			},
		},
		LotsBalanceStrategy: Flat,
		Orders: OrdersConfig{
			SellOutProfit: OrderConfig{
				Type:                Limit,
				ProfitPercentIndent: 0.05,
			},
			SellOrder: OrderConfig{
				Type: Market,
			},
			BuyOrder: OrderConfig{
				Type: Market,
			},
			HedgeOrder: OrderConfig{
				Type:                 Limit,
				DefencePercentIndent: 0.3,
			},
		},
		TechnicalIndicators: func() TechnicalIndicatorsConfig {
			t := TechnicalIndicatorsConfig{}
			t.Setup()
			return t
		}(),
	},
}

func (b BacktestConfig) Validate() error {
	if b.From.After(b.To) {
		return fmt.Errorf("from aftet to")
	}

	return nil
}
