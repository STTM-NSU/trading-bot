package instrument

import (
	"fmt"

	"github.com/STTM-NSU/trading-bot/internal/model"
	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
)

func (s *InstrumentsService) GetEtfs() ([]model.Instrument, error) {
	s.rateLimiter.Take()
	resp, err := s.instrClient.Etfs(investapi.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get currency from instrument", err)
	}

	responseInstruments := make([]model.Instrument, 0, len(resp.GetInstruments()))
	for _, instrument := range resp.GetInstruments() {
		if !instrument.GetApiTradeAvailableFlag() || !instrument.GetSellAvailableFlag() ||
			!instrument.GetBuyAvailableFlag() || instrument.GetBlockedTcaFlag() {
			continue
		}

		responseInstruments = append(responseInstruments, model.Instrument{
			FIGI:              instrument.GetFigi(),
			UID:               instrument.GetUid(),
			ISIN:              instrument.GetIsin(),
			Ticker:            instrument.GetTicker(),
			ClassCode:         instrument.GetClassCode(),
			Lot:               int(instrument.GetLot()),
			Currency:          instrument.GetCurrency(),
			ForQualInvestor:   instrument.GetForQualInvestorFlag(),
			Exchange:          instrument.GetRealExchange().String(),
			ExchangeSection:   instrument.GetExchange(),
			InstrumentType:    model.Etf,
			MinPriceIncrement: instrument.GetMinPriceIncrement().ToFloat(),
		})
	}

	return responseInstruments, nil
}

func (s *InstrumentsService) GetShares() ([]model.Instrument, error) {
	s.rateLimiter.Take()
	resp, err := s.instrClient.Shares(investapi.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get currency from instrument", err)
	}

	responseInstruments := make([]model.Instrument, 0, len(resp.GetInstruments()))
	for _, instrument := range resp.GetInstruments() {
		if !instrument.GetApiTradeAvailableFlag() || !instrument.GetSellAvailableFlag() ||
			!instrument.GetBuyAvailableFlag() || instrument.GetBlockedTcaFlag() {
			continue
		}

		responseInstruments = append(responseInstruments, model.Instrument{
			FIGI:              instrument.GetFigi(),
			UID:               instrument.GetUid(),
			ISIN:              instrument.GetIsin(),
			Ticker:            instrument.GetTicker(),
			ClassCode:         instrument.GetClassCode(),
			Lot:               int(instrument.GetLot()),
			Currency:          instrument.GetCurrency(),
			ForQualInvestor:   instrument.GetForQualInvestorFlag(),
			Exchange:          instrument.GetRealExchange().String(),
			ExchangeSection:   instrument.GetExchange(),
			InstrumentType:    model.Share,
			MinPriceIncrement: instrument.GetMinPriceIncrement().ToFloat(),
		})
	}

	return responseInstruments, nil
}

func (s *InstrumentsService) GetBonds() ([]model.Instrument, error) {
	s.rateLimiter.Take()
	resp, err := s.instrClient.Bonds(investapi.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get currency from instrument", err)
	}

	responseInstruments := make([]model.Instrument, 0, len(resp.GetInstruments()))
	for _, instrument := range resp.GetInstruments() {
		if !instrument.GetApiTradeAvailableFlag() || !instrument.GetSellAvailableFlag() ||
			!instrument.GetBuyAvailableFlag() || instrument.GetBlockedTcaFlag() {
			continue
		}

		responseInstruments = append(responseInstruments, model.Instrument{
			FIGI:              instrument.GetFigi(),
			UID:               instrument.GetUid(),
			ISIN:              instrument.GetIsin(),
			Ticker:            instrument.GetTicker(),
			ClassCode:         instrument.GetClassCode(),
			Lot:               int(instrument.GetLot()),
			Currency:          instrument.GetCurrency(),
			ForQualInvestor:   instrument.GetForQualInvestorFlag(),
			Exchange:          instrument.GetRealExchange().String(),
			ExchangeSection:   instrument.GetExchange(),
			InstrumentType:    model.Bond,
			MinPriceIncrement: instrument.GetMinPriceIncrement().ToFloat(),
		})
	}

	return responseInstruments, nil
}

func (s *InstrumentsService) GetCurrencies() ([]model.Instrument, error) {
	s.rateLimiter.Take()
	resp, err := s.instrClient.Currencies(investapi.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get currency from instrument", err)
	}

	responseInstruments := make([]model.Instrument, 0, len(resp.GetInstruments()))
	for _, instrument := range resp.GetInstruments() {
		if !instrument.GetApiTradeAvailableFlag() || !instrument.GetSellAvailableFlag() ||
			!instrument.GetBuyAvailableFlag() || instrument.GetBlockedTcaFlag() {
			continue
		}

		responseInstruments = append(responseInstruments, model.Instrument{
			FIGI:              instrument.GetFigi(),
			UID:               instrument.GetUid(),
			ISIN:              instrument.GetIsin(),
			Ticker:            instrument.GetTicker(),
			ClassCode:         instrument.GetClassCode(),
			Lot:               int(instrument.GetLot()),
			Currency:          instrument.GetCurrency(),
			ForQualInvestor:   instrument.GetForQualInvestorFlag(),
			Exchange:          instrument.GetRealExchange().String(),
			ExchangeSection:   instrument.GetExchange(),
			InstrumentType:    model.Currency,
			MinPriceIncrement: instrument.GetMinPriceIncrement().ToFloat(),
		})
	}

	return responseInstruments, nil
}
