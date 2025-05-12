package sttm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
	"resty.dev/v3"
)

const (
	_sttmIndexURL = "/get-index"
)

type STTMService struct {
	c   *resty.Client
	cfg config.STTMConfig

	logger logger.Logger
}

func NewSTTMService(cfg config.STTMConfig, logger logger.Logger) *STTMService {
	client := resty.New().
		SetLogger(logger).
		SetBaseURL(cfg.Address)

	return &STTMService{
		c:      client,
		cfg:    cfg,
		logger: logger,
	}
}

func (s *STTMService) GetConfig() config.STTMConfig {
	return s.cfg
}

// curl -X GET "http://192.168.0.24:8000/get-index?instrument_ids=BBG004730N88,BBG004730N88,BBG004730N88&from=2022-11-04T00:00:00&to=2022-11-05T00:00:00&alpha=0.05&p_value=0.05&threshold=0.3" -H "accept: application/json"
func (s *STTMService) GetIndexes(ctx context.Context, from, to time.Time, instrumentIds ...string) ([]float64, time.Duration, error) {
	if from.After(to) {
		return nil, 0, fmt.Errorf("invalid interval")
	}
	if to.Sub(from).Hours() < 24 {
		return nil, 0, fmt.Errorf("interval must be at least one day")
	}
	if len(instrumentIds) == 0 {
		return nil, 0, fmt.Errorf("empty ids")
	}

	fromString := fmt.Sprintf("%s", from.UTC().Format(time.RFC3339))
	fromString = fromString[:len(fromString)-1]
	toString := fmt.Sprintf("%s", to.UTC().Format(time.RFC3339))
	toString = toString[:len(toString)-1]

	req := s.c.R().
		SetQueryParams(map[string]string{
			"from":           fromString,
			"to":             toString,
			"instrument_ids": strings.Join(instrumentIds, ","),
			"alpha":          strconv.FormatFloat(s.cfg.STTMHyperparameters.Alpha, 'f', 2, 64),
			"p_value":        strconv.FormatFloat(s.cfg.STTMHyperparameters.PValue, 'f', 2, 64),
			"threshold":      strconv.FormatFloat(s.cfg.STTMHyperparameters.Threshold, 'f', 2, 64),
		}).
		SetResult(&model.STTMResponse{}).
		SetError(&model.STTMErrorResponse{}).
		SetContext(ctx)

	resp, err := req.Get(_sttmIndexURL)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: can't send request for  sttm index", err)
	}
	defer resp.Body.Close()

	s.logger.Debugf("got response %s status: %s, %s", resp.Request.URL, resp.Status(), resp.Duration())

	if resp.IsError() {
		response := resp.Error().(*model.STTMErrorResponse)
		return nil, response.RetryAfter, fmt.Errorf("%s: sttm index request error", response.Message)
	}
	if resp.IsSuccess() {
		return resp.Result().(*model.STTMResponse).Indexes, 0, nil
	}

	return nil, 0, fmt.Errorf("sttm index unexpected request error: %s", resp.Status())
}
