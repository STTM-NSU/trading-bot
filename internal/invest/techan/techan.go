package techan

import (
	"fmt"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
	"go.uber.org/ratelimit"
)

type TechAnalyseService struct {
	logger      logger.Logger
	mdService   *investgo.MarketDataServiceClient
	rateLimiter ratelimit.Limiter

	cfg config.TechnicalIndicatorsConfig
}

func NewTechAnalyseService(c *investgo.Client, cfg config.TechnicalIndicatorsConfig, logger logger.Logger) *TechAnalyseService {
	return &TechAnalyseService{
		logger:      logger,
		cfg:         cfg,
		rateLimiter: ratelimit.New(200, ratelimit.Per(1*time.Minute)),
		mdService:   c.NewMarketDataServiceClient(),
	}
}

func GetIntervalFromTime(t time.Duration) investapi.GetTechAnalysisRequest_IndicatorInterval {
	switch {
	case t.Hours() >= 7*24:
		return investapi.GetTechAnalysisRequest_INDICATOR_INTERVAL_WEEK
	case t.Hours() >= 24:
		return investapi.GetTechAnalysisRequest_INDICATOR_INTERVAL_ONE_DAY
	case t.Hours() >= 1:
		return investapi.GetTechAnalysisRequest_INDICATOR_INTERVAL_ONE_HOUR
	case t.Minutes() >= 1:
		return investapi.GetTechAnalysisRequest_INDICATOR_INTERVAL_ONE_MINUTE
	default:
		return investapi.GetTechAnalysisRequest_INDICATOR_INTERVAL_ONE_HOUR
	}
}

type TechIndicatorValue struct {
	Value float64

	SlowValue float64
	FastValue float64

	LowerBand float64
	UpperBand float64

	Ts time.Time
}

// instrumentId = UID !!!
func (t *TechAnalyseService) GetRSI(instrumentId string, from, to time.Time) ([]TechIndicatorValue, error) {
	req := &investgo.GetTechAnalysisRequest{
		IndicatorType: investapi.GetTechAnalysisRequest_INDICATOR_TYPE_RSI,
		InstrumentUID: instrumentId,
		From:          from,
		To:            to,
		Interval:      GetIntervalFromTime(t.cfg.RSI.TimeUnit),
		TypeOfPrice:   investapi.GetTechAnalysisRequest_TYPE_OF_PRICE_CLOSE,
		Length:        int32(t.cfg.RSI.Length),
	}

	t.rateLimiter.Take()
	resp, err := t.mdService.GetTechAnalysis(req)
	if err != nil {
		return nil, fmt.Errorf("GetRSI: %w", err)
	}

	techIndicatorValues := make([]TechIndicatorValue, 0, len(resp.GetTechnicalIndicators()))

	for _, i := range resp.GetTechnicalIndicators() {
		techIndicatorValues = append(techIndicatorValues, TechIndicatorValue{
			Value: i.GetSignal().ToFloat(),
			Ts:    i.GetTimestamp().AsTime(),
		})
	}

	return techIndicatorValues, nil
}

func (t *TechAnalyseService) GetBB(instrumentId string, from, to time.Time) ([]TechIndicatorValue, error) {
	req := &investgo.GetTechAnalysisRequest{
		IndicatorType: investapi.GetTechAnalysisRequest_INDICATOR_TYPE_BB,
		InstrumentUID: instrumentId,
		From:          from,
		To:            to,
		Interval:      GetIntervalFromTime(t.cfg.BollingerBands.TimeUnit),
		TypeOfPrice:   investapi.GetTechAnalysisRequest_TYPE_OF_PRICE_CLOSE,
		Length:        int32(t.cfg.BollingerBands.Length),
		Deviation: &investapi.GetTechAnalysisRequest_Deviation{
			DeviationMultiplier: investgo.FloatToQuotation(t.cfg.BollingerBands.Deviation, nil),
		},
	}

	t.rateLimiter.Take()
	resp, err := t.mdService.GetTechAnalysis(req)
	if err != nil {
		return nil, fmt.Errorf("GetRSI: %w", err)
	}

	techIndicatorValues := make([]TechIndicatorValue, 0, len(resp.GetTechnicalIndicators()))

	for _, i := range resp.GetTechnicalIndicators() {
		techIndicatorValues = append(techIndicatorValues, TechIndicatorValue{
			LowerBand: i.GetLowerBand().ToFloat(),
			UpperBand: i.GetUpperBand().ToFloat(),
			Ts:        i.GetTimestamp().AsTime(),
		})
	}

	return techIndicatorValues, nil
}

func (t *TechAnalyseService) GetEMA(instrumentId string, from, to time.Time) ([]TechIndicatorValue, error) {
	reqFast := &investgo.GetTechAnalysisRequest{
		IndicatorType: investapi.GetTechAnalysisRequest_INDICATOR_TYPE_EMA,
		InstrumentUID: instrumentId,
		From:          from,
		To:            to,
		Interval:      GetIntervalFromTime(t.cfg.EMA.TimeUnit),
		TypeOfPrice:   investapi.GetTechAnalysisRequest_TYPE_OF_PRICE_CLOSE,
		Length:        int32(t.cfg.EMA.FastLength),
	}

	reqSlow := &investgo.GetTechAnalysisRequest{
		IndicatorType: investapi.GetTechAnalysisRequest_INDICATOR_TYPE_EMA,
		InstrumentUID: instrumentId,
		From:          from,
		To:            to,
		Interval:      GetIntervalFromTime(t.cfg.EMA.TimeUnit),
		TypeOfPrice:   investapi.GetTechAnalysisRequest_TYPE_OF_PRICE_CLOSE,
		Length:        int32(t.cfg.EMA.FastLength),
	}

	t.rateLimiter.Take()
	respFast, err := t.mdService.GetTechAnalysis(reqFast)
	if err != nil {
		return nil, fmt.Errorf("GetRSI: %w", err)
	}

	t.rateLimiter.Take()
	respSlow, err := t.mdService.GetTechAnalysis(reqSlow)
	if err != nil {
		return nil, fmt.Errorf("GetRSI: %w", err)
	}

	m := make(map[time.Time]TechIndicatorValue, len(respFast.GetTechnicalIndicators()))

	for _, i := range respFast.GetTechnicalIndicators() {
		m[i.GetTimestamp().AsTime()] = TechIndicatorValue{
			FastValue: i.GetSignal().ToFloat(),
			Ts:        i.GetTimestamp().AsTime(),
		}
	}

	for _, i := range respSlow.GetTechnicalIndicators() {
		v, ok := m[i.GetTimestamp().AsTime()]
		if !ok {
			continue
		}
		m[i.GetTimestamp().AsTime()] = TechIndicatorValue{
			SlowValue: i.GetSignal().ToFloat(),
			FastValue: v.FastValue,
			Ts:        v.Ts,
		}
	}

	techIndicatorValues := make([]TechIndicatorValue, 0, len(m))

	for _, v := range m {
		techIndicatorValues = append(techIndicatorValues, v)
	}

	return techIndicatorValues, nil
}

func (t *TechAnalyseService) GetMACD(instrumentId string, from, to time.Time) ([]TechIndicatorValue, error) {
	req := &investgo.GetTechAnalysisRequest{
		IndicatorType: investapi.GetTechAnalysisRequest_INDICATOR_TYPE_MACD,
		InstrumentUID: instrumentId,
		From:          from,
		To:            to,
		Interval:      GetIntervalFromTime(t.cfg.MACD.TimeUnit),
		TypeOfPrice:   investapi.GetTechAnalysisRequest_TYPE_OF_PRICE_CLOSE,
		Smoothing: &investapi.GetTechAnalysisRequest_Smoothing{
			FastLength:      int32(t.cfg.MACD.FastLength),
			SlowLength:      int32(t.cfg.MACD.SlowLength),
			SignalSmoothing: int32(t.cfg.MACD.SignalSmoothing),
		},
	}

	t.rateLimiter.Take()
	resp, err := t.mdService.GetTechAnalysis(req)
	if err != nil {
		return nil, fmt.Errorf("GetRSI: %w", err)
	}

	techIndicatorValues := make([]TechIndicatorValue, 0, len(resp.GetTechnicalIndicators()))

	for _, i := range resp.GetTechnicalIndicators() {
		techIndicatorValues = append(techIndicatorValues, TechIndicatorValue{
			Value: i.GetMacd().ToFloat(),
			Ts:    i.GetTimestamp().AsTime(),
		})
	}

	return techIndicatorValues, nil
}
