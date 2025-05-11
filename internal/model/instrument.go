package model

import (
	"time"

	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
)

type Instrument struct {
	FIGI              string         `json:"figi" db:"id"`
	Ticker            string         `json:"ticker" db:"ticker"`
	ClassCode         string         `json:"class_code" db:"class_code"`
	UID               string         `json:"uid" db:"uid"`
	ISIN              string         `json:"isin" db:"isin"`
	Query             string         `json:"query" db:"query"`
	Lot               int            `json:"lot" db:"lot"`
	Currency          string         `json:"currency" db:"currency"`
	ForQualInvestor   bool           `json:"for_qual_investor" db:"for_qual_investor"`
	Exchange          string         `json:"exchange" db:"exchange"`
	ExchangeSection   string         `json:"exchange_section" db:"exchange_section"`
	InstrumentType    InstrumentType `json:"instrument_type" db:"instrument_type"`
	MinPriceIncrement float64        `json:"min_price_increment" db:"min_price_increment"`
}

func (p Instrument) GetUID() string {
	return p.UID
}

type InstrumentType string

const (
	Bond     InstrumentType = "bond"
	Share    InstrumentType = "share"
	Currency InstrumentType = "currency"
	Etf      InstrumentType = "etf"
)

func (i InstrumentType) ToInvestAPIType() investapi.InstrumentType {
	switch i {
	case Bond:
		return investapi.InstrumentType_INSTRUMENT_TYPE_BOND
	case Share:
		return investapi.InstrumentType_INSTRUMENT_TYPE_SHARE
	case Currency:
		return investapi.InstrumentType_INSTRUMENT_TYPE_CURRENCY
	case Etf:
		return investapi.InstrumentType_INSTRUMENT_TYPE_ETF
	default:
		return investapi.InstrumentType_INSTRUMENT_TYPE_UNSPECIFIED
	}
}

func FromInvestAPIType(t investapi.InstrumentType) InstrumentType {
	switch t {
	case investapi.InstrumentType_INSTRUMENT_TYPE_BOND:
		return Bond
	case investapi.InstrumentType_INSTRUMENT_TYPE_SHARE:
		return Share
	case investapi.InstrumentType_INSTRUMENT_TYPE_CURRENCY:
		return Currency
	case investapi.InstrumentType_INSTRUMENT_TYPE_ETF:
		return Etf
	default:
		return ""
	}
}

func (i InstrumentType) ToAssetType() investapi.AssetType {
	switch i {
	case Bond, Share, Etf:
		return investapi.AssetType_ASSET_TYPE_SECURITY
	case Currency:
		return investapi.AssetType_ASSET_TYPE_CURRENCY
	default:
		return investapi.AssetType_ASSET_TYPE_UNSPECIFIED
	}
}

type TradingSchedule struct {
	Date         time.Time `json:"date"`
	IsTradingDay bool      `json:"is_trading_day"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
}
