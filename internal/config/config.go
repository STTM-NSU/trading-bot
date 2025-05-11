package config

import (
	"fmt"
	"os"

	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

func LoadInvestConfig(filename string) (investgo.Config, error) {
	cfg, err := investgo.LoadConfig(filename)
	if err != nil {
		return investgo.Config{}, fmt.Errorf("%w: can't load config", err)
	}

	cfg.Token = os.Getenv("T_INVEST_API_TOKEN")
	if cfg.Token == "" {
		return investgo.Config{}, fmt.Errorf("empty t-invest api token")
	}

	return cfg, nil
}
