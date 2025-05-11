package techan

import "time"

func (t *TechAnalyseService) GetRSIBBSignal(instrumentId string, price float64, from time.Time) (bool, error) {
	rsiResp, err := t.GetRSI(instrumentId, from.UTC(), from.Add(1*time.Hour).UTC())
	if err != nil {
		return false, err
	}

	if len(rsiResp) == 0 || !rsiResp[0].Ts.Truncate(1*time.Hour).Equal(from) {
		return false, nil
	}

	rsi := rsiResp[0].Value

	bbResp, err := t.GetBB(instrumentId, from.UTC(), from.Add(1*time.Hour).UTC())
	if err != nil {
		return false, err
	}

	if len(bbResp) == 0 || !bbResp[0].Ts.Truncate(1*time.Hour).Equal(from) {
		return false, nil
	}

	bbU, bbL := bbResp[0].UpperBand, bbResp[0].LowerBand

	if rsi > t.cfg.RSI.UpperBound && price > bbU {
		return true, nil
	}
	if rsi < t.cfg.RSI.LowerBound && price < bbL {
		return false, nil
	}

	return false, nil
}

func (t *TechAnalyseService) GetEMAMACDSignal(instrumentId string, price float64, from time.Time) (bool, error) {
	emaResp, err := t.GetEMA(instrumentId, from.UTC(), from.Add(1*time.Hour).UTC())
	if err != nil {
		return false, err
	}
	if len(emaResp) == 0 || !emaResp[0].Ts.Truncate(1*time.Hour).Equal(from) {
		return false, nil
	}

	emaSlow, emaFast := emaResp[0].SlowValue, emaResp[0].FastValue

	macdResp, err := t.GetMACD(instrumentId, from.UTC(), from.Add(1*time.Hour).UTC())
	if err != nil {
		return false, err
	}

	if len(macdResp) == 0 || !macdResp[0].Ts.Truncate(1*time.Hour).Equal(from) {
		return false, nil
	}

	macd := macdResp[0].Value

	if emaSlow > emaFast && macd < 0 {
		return true, nil
	}

	return false, nil
}
