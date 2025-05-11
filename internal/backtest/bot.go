package backtest

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/invest/instrument"
	"github.com/STTM-NSU/trading-bot/internal/invest/md"
	"github.com/STTM-NSU/trading-bot/internal/invest/techan"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
	"github.com/STTM-NSU/trading-bot/internal/sttm"
)

type TradingBot struct {
	logger logger.Logger

	instrumentsService *instrument.InstrumentsService
	cfgInstruments     config.Instruments

	candlesService *md.CandlesService
	techAn         *techan.TechAnalyseService

	sttmService *sttm.STTMService

	executor  *Executor
	ordersCfg config.OrdersConfig

	portfolio *Portfolio
}

func NewTradingBot(logger logger.Logger,
	instrumentsService *instrument.InstrumentsService,
	cfgInstruments config.Instruments,
	candlesService *md.CandlesService,
	techAn *techan.TechAnalyseService,
	sttmService *sttm.STTMService,
	executor *Executor,
	ordersCfg config.OrdersConfig,
	portfolio *Portfolio,
) *TradingBot {
	return &TradingBot{
		logger:             logger,
		instrumentsService: instrumentsService,
		cfgInstruments:     cfgInstruments,
		candlesService:     candlesService,
		techAn:             techAn,
		sttmService:        sttmService,
		executor:           executor,
		ordersCfg:          ordersCfg,
		portfolio:          portfolio,
	}
}

func (t *TradingBot) ExecutorCheck(from time.Time) {
	t.executor.Check(from)
}

func (t *TradingBot) CheckTechIndicators(currentTime time.Time) {
	instruments := t.portfolio.GetInstruments()

	for _, instr := range instruments {
		price, err := t.candlesService.GetLastPriceOn(instr.InstrumentID, currentTime)
		if err != nil {
			t.logger.Errorf("GetLastPriceOn techan check err: %v", err)
			continue
		}

		sellSignalEMAMACD, err := t.techAn.GetEMAMACDSignal(instr.InstrumentID, price, currentTime)
		if err != nil {
			t.logger.Errorf("%s: can't get sell signal ema macd", err)
			continue
		}
		if sellSignalEMAMACD {
			t.logger.Infof("%s: techan signal ema macd", instr.InstrumentID)
			t.executor.SellMarket(instr)
		}

		sellSignalRSIBB, err := t.techAn.GetRSIBBSignal(instr.InstrumentID, price, currentTime)
		if err != nil {
			t.logger.Errorf("%s: can't get sell signal rsi bb", err)
			continue
		}
		if !sellSignalEMAMACD && sellSignalRSIBB {
			t.logger.Infof("%s: techan signal rsi bb", instr.InstrumentID)
			t.executor.SellMarket(instr)
		}
	}
}

func toMap[T interface{ GetUID() string }](arr []T) map[string]T {
	m := make(map[string]T)
	for _, i := range arr {
		m[i.GetUID()] = i
	}
	return m
}

func fromMap[T interface{ GetUID() string }](m map[string]T) []T {
	arr := make([]T, 0, len(m))
	for _, i := range m {
		arr = append(arr, i)
	}
	return arr
}

func GetSellProfitSellBuyInstruments(top []model.Instrument, portMap map[string]model.PortfolioInstrument) (
	[]model.PortfolioInstrument,
	[]model.PortfolioInstrument,
	[]model.Instrument,
) {
	topMap := toMap(top)

	sellProfit := make([]model.PortfolioInstrument, 0)
	sell := make([]model.PortfolioInstrument, 0)
	buy := make([]model.Instrument, 0)

	for id, t := range topMap {
		if v, ok := portMap[id]; ok {
			sellProfit = append(sellProfit, v)
		} else {
			buy = append(buy, t)
		}
	}
	for id, p := range portMap {
		if _, ok := topMap[id]; !ok {
			sell = append(sell, p)
		}
	}

	return sellProfit, sell, buy
}

func (t *TradingBot) SellOutRemaining() {
	t.executor.SellOut()
}

func (t *TradingBot) Rebalance(ctx context.Context, from, to time.Time) error {
	topInstruments, err := t.GetRebalancedTopInstruments(ctx, from, to)
	if err != nil {
		return fmt.Errorf("GetRebalanceTopInstruments: %w", err)
	}
	t.logger.Infof("Top instruments: %v", len(topInstruments))

	portfolioInstruments := t.portfolio.GetInstruments()
	sellProfit, sell, buy := GetSellProfitSellBuyInstruments(topInstruments, portfolioInstruments)
	t.logger.Infof("sellProfit: %v sell: %v buy: %v", len(sellProfit), len(sell), len(buy))
	t.logger.Infof("more info sellProfit: %v sell: %v buy: %v", sellProfit, sell, buy)

	for _, i := range sellProfit {
		lp, err := t.candlesService.GetLastPriceOn(i.InstrumentID, to)
		if err != nil {
			return fmt.Errorf("GetLastPriceOn: %w", err)
		}
		t.executor.SellLimit(lp, t.ordersCfg.SellOutProfit.ProfitPercentIndent, t.ordersCfg.SellOutProfit.DefencePercentIndent, i)
	}
	t.logger.Infof("sellProfit requested")

	for _, i := range sell {
		t.executor.SellMarket(i)
	}

	t.logger.Infof("sellMarket requested")

	if err := t.BuyInstruments(buy, to); err != nil {
		return fmt.Errorf("BuyInstruments: %w", err)
	}
	t.logger.Infof("BuyInstruments requested")
	return nil
}

func (t *TradingBot) BuyInstruments(instruments []model.Instrument, to time.Time) error {
	lastPrices := make(map[string]float64)
	for _, i := range instruments {
		lp, err := t.candlesService.GetLastPriceOn(i.UID, to)
		if err != nil {
			return fmt.Errorf("GetLastPriceOn: %w", err)
		}
		lastPrices[i.UID] = lp
	}
	if len(instruments) == 0 {
		return nil
	}

	sum := 0.0
	for i := 0; true; i++ {
		instr := instruments[i%len(instruments)]
		price := lastPrices[instr.UID] * float64(instr.Lot)
		if sum+price > t.portfolio.GetBalance() {
			break
		}

		t.executor.BuyMarket(1, instr)
		sum += price
	}
	return nil
}

func (t *TradingBot) GetRebalancedTopInstruments(ctx context.Context, from, to time.Time) ([]model.Instrument, error) {
	instrs, err := t.instrumentsService.LoadInstruments(t.cfgInstruments)
	if err != nil {
		return nil, fmt.Errorf("LoadInstruments: %v", err)
	}
	t.logger.Infof("loaded instruments: %v", len(instrs))

	availableBalance := t.portfolio.GetBalance()
	instruments := make([]model.Instrument, 0, len(instrs))
	for _, i := range instrs {
		lastPrice, err := t.candlesService.GetLastPriceOn(i.UID, to)
		if err != nil {
			t.logger.Errorf("GetLastPriceOn: %v", err)
			continue
		}
		if lastPrice*float64(i.Lot) > availableBalance {
			t.logger.Infof("last price: %v > %v", lastPrice*float64(i.Lot), availableBalance)
			continue
		}
		instruments = append(instruments, i)
	}
	t.logger.Infof("sort instruments by price: %v", len(instruments))

	sttmCfg := t.sttmService.GetConfig()
	indexes := make(map[string]float64, len(instruments))
	instrumentsIds := func() []string {
		ids := make([]string, 0, len(instruments))
		for _, i := range instruments {
			ids = append(ids, i.FIGI)
		}
		return ids
	}()
	indexesApi, err := t.retry(ctx, func() ([]float64, time.Duration, error) {
		return t.sttmService.GetIndexes(ctx, from, to.Add(24*time.Hour).Truncate(24*time.Hour), instrumentsIds...)
	})
	if err != nil {
		t.logger.Errorf("GetIndex: %v", err)
		return nil, err
	}

	sttmInstruments := make([]model.Instrument, 0, len(instruments))
	for i, index := range indexesApi {
		t.logger.Infof("index for instrument %s: %v", instruments[i].UID, index)
		if index < sttmCfg.TopSTTMThreshold {
			continue
		}
		indexes[instruments[i].UID] = index
		sttmInstruments = append(sttmInstruments, instruments[i])
	}
	t.logger.Infof("end sttm requests: %v", len(indexes))

	slices.SortFunc(sttmInstruments, func(a, b model.Instrument) int {
		if indexes[a.UID] > indexes[b.UID] {
			return -1
		} else if indexes[a.UID] < indexes[b.UID] {
			return 1
		}
		return 0
	})

	for _, i := range instruments {
		t.logger.Infof("instrument: %v %f", i.FIGI, indexes[i.UID])
	}

	topN := int(float64(len(sttmInstruments)) * sttmCfg.TopSTTMPercent)
	topInstruments := make([]model.Instrument, 0, topN)
	for i := range topN {
		topInstruments = append(topInstruments, sttmInstruments[i])
	}

	return topInstruments, nil
}

func (t *TradingBot) retry(ctx context.Context, f func() ([]float64, time.Duration, error)) ([]float64, error) {
	index, waitFor, err := f()
	if waitFor != 0 {
		t.logger.Infof("retry waiting for %v", waitFor)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitFor):
			return t.retry(ctx, f)
		}
	}
	return index, err
}
