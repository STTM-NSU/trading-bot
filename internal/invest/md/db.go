package md

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/model"
)

const (
	_queryStocks = "SELECT ts, close_price FROM stocks WHERE ts BETWEEN $1::timestamp AND $2::timestamp AND instrument_id = $3 ORDER BY ts DESC"
)

func (s *CandlesService) GetCandlesFromDB(instrumentId string, from, to time.Time) ([]model.Candle, error) {
	var candles []model.Candle
	if err := s.db.Select(&candles, _queryStocks, from, to, instrumentId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get candles from database: %w", err)
	}
	return candles, nil
}
