package config

import "time"

type RSIConfig struct {
	Length     float64       `yaml:"length"`
	TimeUnit   time.Duration `yaml:"time_unit"`
	UpperBound float64       `yaml:"upper_bound"`
	LowerBound float64       `yaml:"lower_bound"`
}

type BollingerBandsConfig struct {
	Length    float64       `yaml:"length"`
	TimeUnit  time.Duration `yaml:"time_unit"`
	Deviation float64       `yaml:"deviation"`
}

type EMAConfig struct {
	FastLength float64       `yaml:"fast_length"`
	SlowLength float64       `yaml:"slow_length"`
	TimeUnit   time.Duration `yaml:"time_unit"`
}

type MACDConfig struct {
	FastLength      float64       `yaml:"fast_length"`
	SlowLength      float64       `yaml:"slow_length"`
	TimeUnit        time.Duration `yaml:"time_unit"`
	SignalSmoothing float64       `yaml:"signal_smoothing"`
}

func (c *TechnicalIndicatorsConfig) Setup() {
	if c.RSI.Length <= 0 {
		c.RSI.Length = 14 * 24
		c.RSI.TimeUnit = 1 * time.Hour
	}
	if c.RSI.TimeUnit <= 0 {
		c.RSI.Length = 14 * 24
		c.RSI.TimeUnit = 1 * time.Hour
	}
	if c.RSI.LowerBound <= 0 {
		c.RSI.LowerBound = 30
	}
	if c.RSI.UpperBound <= 0 {
		c.RSI.UpperBound = 70
	}

	if c.BollingerBands.Deviation <= 0 {
		c.BollingerBands.Deviation = 2
	}
	if c.BollingerBands.Length <= 0 {
		c.BollingerBands.Length = 14 * 24
		c.BollingerBands.TimeUnit = 1 * time.Hour
	}
	if c.BollingerBands.TimeUnit <= 0 {
		c.BollingerBands.Length = 14 * 24
		c.BollingerBands.TimeUnit = 1 * time.Hour
	}

	if c.EMA.SlowLength <= 0 {
		c.EMA.SlowLength = 50
		c.EMA.TimeUnit = 24 * time.Hour
	}
	if c.EMA.FastLength <= 0 {
		c.EMA.FastLength = 20
		c.EMA.TimeUnit = 24 * time.Hour
	}
	if c.EMA.TimeUnit <= 0 {
		c.EMA.FastLength = 20
		c.EMA.SlowLength = 50
		c.EMA.TimeUnit = 24 * time.Hour
	}

	if c.MACD.SignalSmoothing <= 0 {
		c.MACD.SignalSmoothing = 9
	}
	if c.MACD.SlowLength <= 0 {
		c.MACD.SlowLength = 26
		c.MACD.TimeUnit = 24 * time.Hour
	}
	if c.MACD.FastLength <= 0 {
		c.MACD.FastLength = 12
		c.MACD.TimeUnit = 24 * time.Hour
	}
	if c.MACD.TimeUnit <= 0 {
		c.MACD.FastLength = 12
		c.MACD.SlowLength = 26
		c.MACD.TimeUnit = 24 * time.Hour
	}
}

type TechnicalIndicatorsConfig struct {
	RSI            RSIConfig            `yaml:"rsi"`
	BollingerBands BollingerBandsConfig `yaml:"bollinger_bands"`
	EMA            EMAConfig            `yaml:"ema"`
	MACD           MACDConfig           `yaml:"macd"`
}
