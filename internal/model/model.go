package model

type MoneyValue struct {
	Currency string  `yaml:"currency"`
	Value    float64 `yaml:"value"`
}
