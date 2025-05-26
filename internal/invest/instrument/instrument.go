package instrument

import (
	"errors"
	"fmt"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
	"go.uber.org/ratelimit"
)

var (
	NotExistError = errors.New("instrument doesn't exist")
	NotFoundError = errors.New("instrument not found")
)

type InstrumentsService struct {
	instrClient *investgo.InstrumentsServiceClient
	rateLimiter ratelimit.Limiter
	logger      logger.Logger

	queriesInstrumentsCache map[string]*model.Instrument
}

func NewInstrumentsService(client *investgo.Client, logger logger.Logger) *InstrumentsService {
	return &InstrumentsService{
		instrClient:             client.NewInstrumentsServiceClient(),
		rateLimiter:             ratelimit.New(200, ratelimit.Per(1*time.Minute)),
		logger:                  logger,
		queriesInstrumentsCache: make(map[string]*model.Instrument),
	}
}

func (s *InstrumentsService) LoadInstruments(cfg config.Instruments) ([]model.Instrument, error) {
	if len(cfg.IDs) > 0 {
		return s.GetInstruments(cfg.IDs...)
	}
	if len(cfg.Types) > 0 {
		return s.GetInstrumentsWithTypes(cfg.Types...)
	}
	return nil, NotExistError
}

func (s *InstrumentsService) GetInstruments(queries ...string) ([]model.Instrument, error) {
	instruments := make([]model.Instrument, 0, len(queries))
	for _, q := range queries {
		i, err := s.GetInstrument(q)
		if err != nil {
			return nil, fmt.Errorf("%w: can't get instrument %s", err, q)
		}
		if i != nil {
			instruments = append(instruments, *i)
		}
	}

	return instruments, nil
}

func (s *InstrumentsService) GetInstrument(query string) (*model.Instrument, error) {
	if v, ok := s.queriesInstrumentsCache[query]; ok && v != nil {
		return v, nil
	}

	s.rateLimiter.Take()
	resp, err := s.instrClient.FindInstrument(query)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get instrument", err)
	}

	instruments := resp.GetInstruments()

	if len(instruments) == 0 {
		return nil, NotExistError
	}

	for _, instrument := range instruments {
		if !instrument.GetApiTradeAvailableFlag() {
			continue
		}

		info, err := s.GetInstrumentInfo(instrument.GetFigi())
		if err != nil {
			s.logger.Warnf("%s: can't get info for figi=%s", err, instrument.GetFigi())
			continue
		}

		if !info.GetBuyAvailableFlag() || !info.GetSellAvailableFlag() || info.GetBlockedTcaFlag() {
			continue
		}

		instr := &model.Instrument{
			FIGI:              info.GetFigi(),
			UID:               info.GetUid(),
			ISIN:              info.GetIsin(),
			Ticker:            info.GetTicker(),
			ClassCode:         info.GetClassCode(),
			Query:             query,
			Lot:               int(info.GetLot()),
			Currency:          info.GetCurrency(),
			ForQualInvestor:   info.GetForQualInvestorFlag(),
			Exchange:          info.GetRealExchange().String(),
			ExchangeSection:   info.GetExchange(),
			InstrumentType:    model.FromInvestAPIType(info.GetInstrumentKind()),
			MinPriceIncrement: info.GetMinPriceIncrement().ToFloat(),
		}

		s.queriesInstrumentsCache[query] = instr

		return instr, nil
	}

	return nil, NotFoundError
}

func (s *InstrumentsService) GetInstrumentsWithTypes(types ...model.InstrumentType) ([]model.Instrument, error) {
	responseInstruments := make([]model.Instrument, 0, 100)
	for _, t := range types {
		switch t {
		case model.Etf:
			i, err := s.GetEtfs()
			if err != nil {
				s.logger.Warnf("%s: can't get currencies", err)
				continue
			}
			responseInstruments = append(responseInstruments, i...)
		case model.Share:
			i, err := s.GetShares()
			if err != nil {
				s.logger.Warnf("%s: can't get currencies", err)
				continue
			}
			responseInstruments = append(responseInstruments, i...)
		case model.Bond:
			i, err := s.GetBonds()
			if err != nil {
				s.logger.Warnf("%s: can't get currencies", err)
				continue
			}
			responseInstruments = append(responseInstruments, i...)
		case model.Currency:
			i, err := s.GetCurrencies()
			if err != nil {
				s.logger.Warnf("%s: can't get currencies", err)
				continue
			}
			responseInstruments = append(responseInstruments, i...)
		}
	}
	return responseInstruments, nil
}

func (s *InstrumentsService) UpdateInstrumentInfo(instrument model.Instrument) (model.Instrument, error) {
	info, err := s.GetInstrumentInfo(instrument.FIGI)
	if err != nil {
		return model.Instrument{}, fmt.Errorf("%w: can't get info for figi", err)
	}

	newInstrument := model.Instrument{
		FIGI:              info.GetFigi(),
		UID:               info.GetUid(),
		ISIN:              info.GetIsin(),
		Ticker:            info.GetTicker(),
		ClassCode:         info.GetClassCode(),
		Lot:               int(info.GetLot()),
		Currency:          info.GetCurrency(),
		ForQualInvestor:   info.GetForQualInvestorFlag(),
		Exchange:          info.GetRealExchange().String(),
		ExchangeSection:   info.GetExchange(),
		InstrumentType:    model.FromInvestAPIType(info.GetInstrumentKind()),
		MinPriceIncrement: info.GetMinPriceIncrement().ToFloat(),
	}

	return newInstrument, nil
}

func (s *InstrumentsService) GetInstrumentInfo(figi string) (*investapi.Instrument, error) {
	s.rateLimiter.Take()
	resp, err := s.instrClient.InstrumentByFigi(figi)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get instrument by figi", err)
	}

	return resp.GetInstrument(), nil
}

func (s *InstrumentsService) GetInstrumentTradingSchedule(i model.Instrument, from, to time.Time) ([]model.TradingSchedule, error) {
	if time.Now().UTC().After(from) {
		return nil, fmt.Errorf("from is in the future")
	}
	if to.Sub(from) >= 7*24*time.Hour {
		return nil, fmt.Errorf("interval must be at least 7 days")
	}
	if i.ExchangeSection == "" {
		return nil, fmt.Errorf("instrument doesn't have an exchange section")
	}

	s.rateLimiter.Take()
	resp, err := s.instrClient.TradingSchedules(i.ExchangeSection, from, to)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get trading schedules", err)
	}

	if len(resp.GetExchanges()) == 0 {
		return nil, fmt.Errorf("empty trading schedules")
	}

	exchangeData := resp.GetExchanges()[0]

	schedules := make([]model.TradingSchedule, 0, len(exchangeData.GetDays()))

	for _, day := range exchangeData.GetDays() {
		schedules = append(schedules, model.TradingSchedule{
			Date:         day.GetDate().AsTime(),
			IsTradingDay: day.GetIsTradingDay(),
			StartDate:    day.GetStartTime().AsTime(),
			EndDate:      day.GetEndTime().AsTime(),
		})
	}

	return schedules, nil
}
