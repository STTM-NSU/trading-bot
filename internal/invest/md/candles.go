package md

import (
	"fmt"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
	"github.com/jmoiron/sqlx"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
	"go.uber.org/ratelimit"
)

type CandlesService struct {
	db     *sqlx.DB
	logger logger.Logger

	rateLimiter ratelimit.Limiter // 600 T/M но мы сделаем меньше

	mdService *investgo.MarketDataServiceClient
}

func NewCandlesService(c *investgo.Client, db *sqlx.DB, logger logger.Logger) *CandlesService {
	return &CandlesService{
		mdService:   c.NewMarketDataServiceClient(),
		rateLimiter: ratelimit.New(500, ratelimit.Per(1*time.Minute)),
		db:          db,
		logger:      logger,
	}
}

func (s *CandlesService) GetLastPriceOn(instrumentId string, from time.Time) (float64, error) {
	candles, err := s.GetCandlesFor(instrumentId, from.Add(-1*time.Hour), from.Add(1*time.Hour))
	if err != nil || len(candles) == 0 {
		return 0, err
	}

	var lastCandle float64
	for _, candle := range candles {
		if candle.Ts.Truncate(24 * time.Hour).Equal(from.Truncate(24 * time.Hour)) {
			lastCandle = candle.ClosePrice
		}
		if candle.Ts.Equal(from) {
			return candle.ClosePrice, nil
		}
	}

	if lastCandle == 0 {
		return 0, fmt.Errorf("no candle %s %s %s", instrumentId, from.Add(-1*time.Hour), from.Add(1*time.Hour))
	}

	return lastCandle, nil
}

func (s *CandlesService) GetLastPrice(instrumentId string) (float64, error) {
	s.rateLimiter.Take()
	resp, err := s.mdService.GetLastPrices([]string{instrumentId})
	if err != nil {
		return 0, fmt.Errorf("can't get last price: %w", err)
	}

	if len(resp.GetLastPrices()) == 0 {
		return 0, fmt.Errorf("empty last price for instrument %s", instrumentId)
	}

	return resp.GetLastPrices()[0].GetPrice().ToFloat(), nil
}

// from to UTC
func (s *CandlesService) GetCandlesFor(instrumentId string, from, to time.Time) ([]model.Candle, error) {
	// f := from.Truncate(time.Hour)
	// t := to.Truncate(time.Hour)
	// hoursCnt := int(to.Sub(from).Hours())
	dbCandles, err := s.GetCandlesFromDB(instrumentId, from, to)
	if err != nil {
		s.logger.Errorf("can't get candles from database: %s", err)
	}

	if len(dbCandles) > 0 {
		return dbCandles, nil
	}
	s.logger.Warnf("no candles for instrument %s from %s to %s", instrumentId, from, to)

	s.rateLimiter.Take()
	resp, err := s.mdService.GetCandles(instrumentId, investapi.CandleInterval_CANDLE_INTERVAL_HOUR, from, to, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("can't get candles from api: %w", err)
	}

	if len(resp.GetCandles()) == 0 {
		return nil, fmt.Errorf("empty candles from api")
	}

	candlesApi := make([]model.Candle, len(resp.GetCandles()))
	for i, item := range resp.GetCandles() {
		candlesApi[i] = model.Candle{
			Ts:         item.GetTime().AsTime(),
			ClosePrice: item.GetClose().ToFloat(),
		}
	}

	return candlesApi, nil
}
