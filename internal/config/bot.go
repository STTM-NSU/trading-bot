package config

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/model"
	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
	"gopkg.in/yaml.v3"
)

type Instruments struct {
	IDs   []string               `yaml:"ids"` // ISINs, FIGIs, ticker + classCodes
	Types []model.InstrumentType `yaml:"types"`
}

type LotsBalanceStrategy string

const (
	Flat    LotsBalanceStrategy = "flat"
	Growing LotsBalanceStrategy = "growing"
)

type CalculationInterval string

const (
	Day  CalculationInterval = "day"
	Week CalculationInterval = "week"
)

type STTMHyperparameters struct {
	Alpha     float64 `yaml:"alpha"`
	PValue    float64 `yaml:"p_value"`
	Threshold float64 `yaml:"threshold"`
}

type STTMConfig struct {
	Address             string              `yaml:"address"`
	TopSTTMPercent      float64             `yaml:"top_sttm_percent"`
	TopSTTMThreshold    float64             `yaml:"top_sttm_treshold"`
	CalculationInterval CalculationInterval `yaml:"calculation_interval"`
	STTMHyperparameters STTMHyperparameters `yaml:"sttm_hyperparameters"`
}

const (
	_topSTTMPercentDefault          = 0.2
	_topSTTMPercentThresholdDefault = 0.2
	_calculationIntervalDefault     = Week
	_alphaDefault                   = 0.05
	_pValueDefault                  = 0.05
	_thresholdDefault               = 0.3
)

func (c *STTMConfig) Setup() error {
	if c.Address == "" {
		return fmt.Errorf("address is required")
	}

	if _, err := url.Parse(c.Address); err != nil {
		return err
	}

	if c.TopSTTMPercent <= 0 {
		c.TopSTTMPercent = _topSTTMPercentDefault
	}
	if c.TopSTTMThreshold <= 0 {
		c.TopSTTMPercent = _topSTTMPercentThresholdDefault
	}
	if c.CalculationInterval == "" {
		c.CalculationInterval = _calculationIntervalDefault
	}
	if c.STTMHyperparameters.Alpha <= 0 {
		c.STTMHyperparameters.Alpha = _alphaDefault
	}
	if c.STTMHyperparameters.PValue <= 0 {
		c.STTMHyperparameters.PValue = _pValueDefault
	}
	if c.STTMHyperparameters.Threshold <= 0 {
		c.STTMHyperparameters.Threshold = _thresholdDefault
	}

	return nil
}

type OrderType string

const (
	Market     OrderType = "market"
	Limit      OrderType = "limit"
	StopLoss   OrderType = "stopLoss"
	StopLimit  OrderType = "stopLimit"
	TakeProfit OrderType = "takeProfit"
)

func (o OrderType) ToInvestType() investapi.OrderType {
	switch o {
	case Market:
		return investapi.OrderType_ORDER_TYPE_MARKET
	case Limit:
		return investapi.OrderType_ORDER_TYPE_LIMIT
	default:
		return investapi.OrderType_ORDER_TYPE_UNSPECIFIED
	}
}

func (o OrderType) ToInvestStopType() investapi.StopOrderType {
	switch o {
	case StopLoss:
		return investapi.StopOrderType_STOP_ORDER_TYPE_STOP_LOSS
	case StopLimit:
		return investapi.StopOrderType_STOP_ORDER_TYPE_STOP_LIMIT
	case TakeProfit:
		return investapi.StopOrderType_STOP_ORDER_TYPE_TAKE_PROFIT
	default:
		return investapi.StopOrderType_STOP_ORDER_TYPE_UNSPECIFIED
	}
}

type OrderConfig struct {
	Type                 OrderType     `yaml:"type"`
	ProfitPercentIndent  float64       `yaml:"max_percent_indent"`
	DefencePercentIndent float64       `yaml:"min_percent_indent"`
	Timeout              time.Duration `yaml:"timeout"`
}

type OrdersConfig struct {
	SellOutProfit OrderConfig `yaml:"sell_out_profit"` // sell if instrument in top appears for second time
	SellOrder     OrderConfig `yaml:"sell_order"`      // for instruments that are out of top
	BuyOrder      OrderConfig `yaml:"buy_order"`       // for buy on rebalance
	HedgeOrder    OrderConfig `yaml:"hedge_order"`     // places with buying order for hedging
}

func (c *OrdersConfig) Setup(isNotSandbox bool) {
	if c.SellOutProfit.Type == "" {
		c.SellOutProfit.Type = TakeProfit
		if !isNotSandbox {
			c.SellOutProfit.Type = Limit
		}
	}
	if c.SellOutProfit.ProfitPercentIndent <= 0 {
		c.SellOutProfit.ProfitPercentIndent = 0.05 // take profit activation value
	}
	if c.SellOutProfit.DefencePercentIndent <= 0 {
		c.SellOutProfit.DefencePercentIndent = 0.05 // close price * value -> post stop loss order
	}
	if c.SellOutProfit.Timeout <= 0 {
		c.SellOutProfit.Timeout = 12 * time.Hour // stop loss
	}

	if c.SellOrder.Type == "" {
		c.SellOrder.Type = StopLimit
		if !isNotSandbox {
			c.SellOrder.Type = Limit
		}
	}
	if c.SellOrder.DefencePercentIndent <= 0 {
		c.SellOrder.DefencePercentIndent = 0.05 // if value is greater than close price * value -> market order
	}
	if c.SellOrder.ProfitPercentIndent <= 0 {
		c.SellOrder.ProfitPercentIndent = 0.05 // actual stop limit price
	}
	if c.SellOrder.Timeout <= 0 {
		c.SellOrder.Timeout = 1 * time.Hour // market
	}

	if c.BuyOrder.Type == "" {
		c.BuyOrder.Type = Limit
	}
	if c.BuyOrder.DefencePercentIndent <= 0 {
		c.BuyOrder.DefencePercentIndent = 0.05 // close price * value -> when buy with just market
	}
	if c.BuyOrder.ProfitPercentIndent <= 0 {
		c.BuyOrder.ProfitPercentIndent = 0.05 // actual order price
	}
	if c.BuyOrder.Timeout <= 0 {
		c.BuyOrder.Timeout = 1 * time.Hour // market
	}

	if c.HedgeOrder.Type == "" {
		c.HedgeOrder.Type = StopLimit
		if !isNotSandbox {
			c.HedgeOrder.Type = Limit
		}
	}
	if c.HedgeOrder.DefencePercentIndent <= 0 {
		c.HedgeOrder.DefencePercentIndent = 0.5 // stop limit value
	}
}

type TradingBotConfig struct {
	IsNotSandbox        bool                      `yaml:"is_not_sandbox"`
	StartAmountOfMoney  []model.MoneyValue        `yaml:"start_amount_of_money"`
	Instruments         Instruments               `yaml:"instruments"`
	STTM                STTMConfig                `yaml:"sttm"`
	LotsBalanceStrategy LotsBalanceStrategy       `yaml:"lots_balance_strategy"`
	Orders              OrdersConfig              `yaml:"orders"`
	TechnicalIndicators TechnicalIndicatorsConfig `yaml:"technical_indicators"`
}

const (
	_lotsBalanceStrategyDefault = Flat
)

func (c *TradingBotConfig) ValidateAndSetup() error {
	for i := range c.StartAmountOfMoney {
		if c.StartAmountOfMoney[i].Value != 0 && c.StartAmountOfMoney[i].Currency == "" {
			return fmt.Errorf("empty currency for start ammount of money")
		}
		if c.StartAmountOfMoney[i].Value < 0 {
			c.StartAmountOfMoney[i].Value = 0
		}
	}

	if len(c.Instruments.IDs) == 0 && len(c.Instruments.Types) == 0 {
		return fmt.Errorf("empty instruments")
	}

	if err := c.STTM.Setup(); err != nil {
		return fmt.Errorf("%w: can't setup sttm", err)
	}

	if c.LotsBalanceStrategy == "" {
		c.LotsBalanceStrategy = _lotsBalanceStrategyDefault
	}

	c.Orders.Setup(c.IsNotSandbox)
	c.TechnicalIndicators.Setup()

	return nil
}

func LoadTradingBotConfig(filename string) (TradingBotConfig, error) {
	var cfg TradingBotConfig
	input, err := os.ReadFile(filename)
	if err != nil {
		return cfg, fmt.Errorf("%w: can't read file", err)
	}

	if err := yaml.Unmarshal(input, &cfg); err != nil {
		return cfg, fmt.Errorf("%w: can't unmarshal config", err)
	}

	if err := cfg.ValidateAndSetup(); err != nil {
		return cfg, fmt.Errorf("%w: can't setup cfg", err)
	}

	return cfg, nil
}
