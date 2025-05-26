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

	marginCfg config.MarginTradingConfig

	portfolio *Portfolio

	lastRebalanceIndexes map[string]float64
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
	marginCfg config.MarginTradingConfig,
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
		marginCfg:          marginCfg,
	}
}

func (t *TradingBot) BuyDeptMargin() {
	t.executor.BuyDeptMargin()
}

func (t *TradingBot) ExecutorCheck(from time.Time) {
	t.executor.CheckTogether(from)
	// t.executor.Check(from)
}

func (t *TradingBot) CheckTechIndicators(currentTime time.Time) {
	instruments := t.portfolio.GetInstruments()

	for _, instr := range instruments {
		// price, err := t.candlesService.GetLastPriceOn(instr.FIGI, currentTime)
		price, err := t.candlesService.GetLastPriceOnDB(instr.FIGI, currentTime)
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
			switch t.ordersCfg.SellOrder.Type {
			case config.Market:
				t.executor.SellMarket(instr)
			case config.Limit:
				t.executor.SellLimit(t.ordersCfg.SellOrder.ProfitPercentIndent, t.ordersCfg.SellOrder.DefencePercentIndent, instr)
			}
		}

		sellSignalRSIBB, err := t.techAn.GetRSIBBSignal(instr.InstrumentID, price, currentTime)
		if err != nil {
			t.logger.Errorf("%s: can't get sell signal rsi bb", err)
			continue
		}
		if !sellSignalEMAMACD && sellSignalRSIBB {
			t.logger.Infof("%s: techan signal rsi bb", instr.InstrumentID)
			switch t.ordersCfg.SellOrder.Type {
			case config.Market:
				t.executor.SellMarket(instr)
			case config.Limit:
				t.executor.SellLimit(t.ordersCfg.SellOrder.ProfitPercentIndent, t.ordersCfg.SellOrder.DefencePercentIndent, instr)
			}
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

func (t *TradingBot) GetInfo() []IntervalProfit {
	return t.executor.GetInfo()
}

func (t *TradingBot) SellOutPortfolio() {
	t.executor.RemoveBuyOrders()
	t.executor.SellOutPortfolio()
}

func (t *TradingBot) SellOutRemaining() {
	t.executor.SellOut()
}

func (t *TradingBot) Rebalance(ctx context.Context, from, to time.Time) error {
	topCasualInstruments, topMarginInstruments, err := t.GetRebalancedTopInstruments(ctx, from, to)
	if err != nil {
		return fmt.Errorf("GetRebalanceTopInstruments: %w", err)
	}
	t.logger.Infof("Top instruments: %v", len(topCasualInstruments))
	if t.marginCfg.Enabled {
		t.logger.Infof("Margin instruments: %v", len(topMarginInstruments))
	}

	portfolioInstruments := t.portfolio.GetInstruments()
	sellProfit, sell, buy := GetSellProfitSellBuyInstruments(topCasualInstruments, portfolioInstruments)
	t.logger.Infof("sellProfit: %v sell: %v buy: %v", len(sellProfit), len(sell), len(buy))
	t.logger.Infof("more info sellProfit: %v sell: %v buy: %v", sellProfit, sell, buy)

	for _, i := range sellProfit {
		t.executor.SellLimit(t.ordersCfg.SellOutProfit.ProfitPercentIndent, t.ordersCfg.SellOutProfit.DefencePercentIndent, i)
	}
	t.logger.Infof("sellProfit requested")

	for _, i := range sell {
		switch t.ordersCfg.SellOrder.Type {
		case config.Market:
			t.executor.SellMarket(i)
		case config.Limit:
			t.executor.SellLimit(t.ordersCfg.SellOrder.ProfitPercentIndent, t.ordersCfg.SellOrder.DefencePercentIndent, i)
		}
	}

	t.logger.Infof("sellMarket requested")

	t.executor.RemoveBuyOrders()
	if err := t.BuyInstruments(buy, to); err != nil {
		return fmt.Errorf("BuyInstruments: %w", err)
	}
	t.logger.Infof("BuyInstruments requested")

	if t.marginCfg.Enabled {
		if err := t.MarginSell(topMarginInstruments, to); err != nil {
			return fmt.Errorf("MarginSell: %w", err)
		}
		t.logger.Infof("MarginSell requested")
	}

	return nil
}

func (t *TradingBot) MarginSell(instruments []model.Instrument, to time.Time) error {
	lastPrices := make(map[string]float64)
	for _, i := range instruments {
		lp, err := t.candlesService.GetLastPriceOn(i.FIGI, to)
		if err != nil {
			t.logger.Errorf("GetLastPriceOn margin sell: %s", err)
			continue
		}
		lastPrices[i.UID] = lp
	}
	if len(instruments) == 0 {
		return nil
	}

	instrumentsQuantities := make(map[string]float64, len(instruments))
	sum := 0.0
	for i := 0; true; i++ {
		instr := instruments[i%len(instruments)]
		if _, ok := lastPrices[instr.UID]; !ok {
			continue
		}
		price := lastPrices[instr.UID] * float64(instr.Lot)
		if sum+price > t.portfolio.GetBalance() {
			break
		}

		instrumentsQuantities[instr.UID]++
		sum += price
	}

	for _, instr := range instruments {
		t.executor.SellMargin(max(instrumentsQuantities[instr.UID]-1, 0),
			t.marginCfg.ShortProfitPercent, t.marginCfg.HedgePercent, instr)
	}

	return nil
}

func (t *TradingBot) BuyInstruments(instruments []model.Instrument, to time.Time) error {
	lastPrices := make(map[string]float64)
	for _, i := range instruments {
		lp, err := t.candlesService.GetLastPriceOn(i.FIGI, to)
		if err != nil {
			t.logger.Errorf("GetLastPriceOn buy instr: %s", err)
			continue
		}
		lastPrices[i.UID] = lp
	}
	if len(instruments) == 0 {
		return nil
	}

	sum := 0.0
	for i := 0; true; i++ {
		instr := instruments[i%len(instruments)]
		if _, ok := lastPrices[instr.UID]; !ok {
			continue
		}
		price := lastPrices[instr.UID] * float64(instr.Lot)
		if sum+price > t.portfolio.GetBalance() {
			break
		}

		t.executor.BuyMarket(1, instr)
		sum += price
	}
	return nil
}

// GetRebalancedTopInstruments returns top for casual trading and for margin trading
func (t *TradingBot) GetRebalancedTopInstruments(ctx context.Context, from, to time.Time) ([]model.Instrument, []model.Instrument, error) {
	instrs, err := t.instrumentsService.LoadInstruments(t.cfgInstruments)
	if err != nil {
		return nil, nil, fmt.Errorf("LoadInstruments: %v", err)
	}
	t.logger.Infof("loaded instruments: %v", len(instrs))

	portfolioInstruments := t.portfolio.GetInstruments()
	availableBalance := t.portfolio.GetBalance()

	// get instruments that we available to buy
	instruments := make([]model.Instrument, 0, len(instrs))
	for _, i := range instrs {
		lastPrice, err := t.candlesService.GetLastPriceOn(i.FIGI, to)
		if err != nil {
			// t.logger.Errorf("GetLastPriceOn: %v", err)
			continue
		}
		if _, ok := portfolioInstruments[i.UID]; lastPrice*float64(i.Lot) > availableBalance && !ok {
			t.logger.Infof("last price: %v > %v", lastPrice*float64(i.Lot), availableBalance)
			continue
		}
		instruments = append(instruments, i)
	}

	t.logger.Infof("try to get sttm indexes for %v instruments", len(instruments))
	sttmCfg := t.sttmService.GetConfig()

	// get STTM indexes for instruments ids
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
		return nil, nil, err
	}

	t.logger.Infof("got sttm indexes: %v", len(indexesApi))

	for i, index := range indexesApi {
		indexes[instruments[i].UID] = index
	}

	slices.SortFunc(instruments, func(a, b model.Instrument) int {
		if indexes[a.UID] > indexes[b.UID] {
			return -1
		} else if indexes[a.UID] < indexes[b.UID] {
			return 1
		}
		return 0
	})

	for _, i := range instruments {
		t.logger.Infof("instrument index: %v = %f", i.FIGI, indexes[i.UID])
	}

	// top percent is calculated relative to overall number of instruments
	topNCasual := int(float64(len(instruments)) * sttmCfg.TopSTTMPercent) // less than instruments len

	topCasualInstruments := make([]model.Instrument, 0, topNCasual)
	for i := range topNCasual {
		if indexes[instruments[i].UID] < sttmCfg.TopSTTMThreshold {
			continue
		}
		topCasualInstruments = append(topCasualInstruments, instruments[i])
	}
	t.logger.Infof("top casual indexes [%d of %d]: %v", len(topCasualInstruments), len(instruments), topCasualInstruments)

	if t.marginCfg.Enabled {
		if t.lastRebalanceIndexes == nil {
			t.lastRebalanceIndexes = indexes
			return topCasualInstruments, nil, nil
		}
		topNMargin := int(float64(len(instruments)) * t.marginCfg.STTMTop)
		topMarginInstruments := make([]model.Instrument, 0, topNMargin)
		for i := range topNMargin {
			idx := len(instruments) - 1 - i
			// we need instruments that were greater than STTMUpperThreshold last time
			if t.lastRebalanceIndexes[instruments[idx].UID] < t.marginCfg.STTMUpperThreshold {
				continue
			}
			// but now lower than STTMThreshold
			if indexes[instruments[idx].UID] > t.marginCfg.STTMThreshold {
				continue
			}
			topMarginInstruments = append(topMarginInstruments, instruments[idx])
		}
		t.lastRebalanceIndexes = indexes
		t.logger.Infof("top margin indexes [%d of %d]: %v", len(topMarginInstruments), len(instruments), topMarginInstruments)
		return topCasualInstruments, topMarginInstruments, nil
	}

	return topCasualInstruments, nil, nil
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
