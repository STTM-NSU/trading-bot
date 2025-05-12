package tools

import (
	"math"

	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
	"github.com/shopspring/decimal"
)

const BILLION int64 = 1000000000

func FloatToQuotation(number float64, step float64) *investapi.Quotation {
	// делим дробь на дробь и округляем до ближайшего целого
	k := math.Round(number / step)
	// целое умножаем на дробный шаг и получаем готовое дробное значение
	roundedNumber := step * k
	// разделяем дробную и целую части
	decNumber := decimal.NewFromFloat(roundedNumber)

	intPart := decNumber.IntPart()
	fracPart := decNumber.Sub(decimal.NewFromInt(intPart))

	nano := fracPart.Mul(decimal.NewFromInt(BILLION)).IntPart()
	return &investapi.Quotation{
		Units: intPart,
		Nano:  int32(nano),
	}
}
