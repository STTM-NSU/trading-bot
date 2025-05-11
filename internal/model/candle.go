package model

import "time"

type Candle struct {
	Ts         time.Time `db:"ts"`
	ClosePrice float64   `db:"close_price"`
}
