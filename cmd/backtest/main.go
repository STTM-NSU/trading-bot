package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/backtest"
	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/invest/instrument"
	"github.com/STTM-NSU/trading-bot/internal/invest/md"
	"github.com/STTM-NSU/trading-bot/internal/invest/techan"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/postgres"
	"github.com/STTM-NSU/trading-bot/internal/sttm"
	"github.com/joho/godotenv"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

const (
	_investCfgFilePath = "./configs/invest.yaml"
)

// from to (test interval)
// start amount of money? - надо только в начале, чтобы собрать портфель, потом просто считаем счёт и разницу с начальным портфелем
// instruments_ids - начальный портфель инструментов
// конфиг sttm
// lots balance strat
// orders config используем как коэффициенты для цен продажи и покупки, только маркет или лимит, имитируем покупку и отсутствие её в течение дня (24 часа)
// технические индикаторы дёргаю с апи и смотрю, мб пора раньше продать, только для этого
// учитываем комиссию

func main() {
	zapLogger, loggerSync, err := logger.NewZapLogger(logger.Info)
	if err != nil {
		log.Fatalf("%s: can't init logger", err)
	}
	defer loggerSync()

	if err := godotenv.Load(); err != nil {
		zapLogger.Warnf("can't detect .env file")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pgConfig := postgres.NewConfigFromEnv().Setup()
	zapLogger.Debugf("trying to connect to db with: %s", pgConfig)
	db, err := postgres.NewDB(pgConfig)
	if err != nil {
		zapLogger.Fatalf("%s: can't connect to db", err)
	}

	investCfg, err := config.LoadInvestConfig(_investCfgFilePath)
	if err != nil {
		zapLogger.Fatalf("%s: can't load invest cfg", err)
	}

	investClient, err := investgo.NewClient(ctx, investCfg, zapLogger)
	if err != nil {
		zapLogger.Fatalf("%s: can't create invest client", err)
	}

	cfg := config.BacktestCfg
	if err := cfg.Validate(); err != nil {
		zapLogger.Fatalf("%s: config validation failed", err)
	}

	candlesService := md.NewCandlesService(investClient, db, zapLogger)
	instrumentsService := instrument.NewInstrumentsService(investClient, zapLogger)
	techAnService := techan.NewTechAnalyseService(investClient, cfg.TechnicalIndicators, zapLogger)
	sttmService := sttm.NewSTTMService(cfg.STTM, zapLogger)

	// собрать стартовый портфель на стартовую сумму - не надо, дождёмся пятницы
	portfolio := backtest.NewPortfolio(zapLogger, cfg.StartAmountOfMoney[0].Value, candlesService)
	executor := backtest.NewExecutor(zapLogger, cfg.Taxes, cfg.MarginTaxes, candlesService, portfolio, cfg.Orders)

	tradingBot := backtest.NewTradingBot(
		zapLogger, instrumentsService, cfg.Instruments, candlesService, techAnService,
		sttmService, executor, cfg.Orders, portfolio, cfg.MarginTradingConfig,
	)

	intervals := backtest.SplitIntoWeeks(cfg.From.UTC(), cfg.To.UTC())
	for i, interval := range intervals { // iterate over weeks
		zapLogger.Infof("Interval: %v", interval)
		zapLogger.Infof("Balance: %v", portfolio.GetBalance())
		zapLogger.Infof("Balance with instruments: %v", portfolio.GetBalanceWithInstruments(interval.Start))
		zapLogger.Infof("Portfolio: %v", portfolio.GetInstruments())
		var lastDay time.Time
		for _, h := range backtest.DivideIntoHours(interval.Start, interval.End) {
			if ctx.Err() != nil {
				break
			}
			if h.Weekday() == time.Saturday || h.Weekday() == time.Sunday {
				continue
			}

			if lastDay != h.Truncate(24*time.Hour) {
				zapLogger.Infof("Day: %v", h)
				lastDay = h.Truncate(24 * time.Hour)
			}

			switch {
			case h.Weekday() == time.Friday && h.Hour() == 18 && i == len(intervals)-1:
				tradingBot.SellOutPortfolio()
			case h.Weekday() == time.Friday && h.Hour() == 19:
				tradingBot.SellOutRemaining()
				tradingBot.BuyDeptMargin()
			case h.Weekday() == time.Friday && h.Hour() == 20 && i != len(intervals)-1:
				zapLogger.Infof("Rebalance on: %s", h)
				if err := tradingBot.Rebalance(ctx, interval.Start, h); err != nil {
					zapLogger.Errorf("%s: rebalance failed", err)
				}
				continue
			}

			if h.Hour() == 12 {
				tradingBot.CheckTechIndicators(h)
			}
			tradingBot.ExecutorCheck(h)
		}
	}

	zapLogger.Infof("Trading bot finished trades")
	zapLogger.Infof("Balance: %v", portfolio.GetBalance())
	zapLogger.Infof("Balance with instruments: %v", portfolio.GetBalanceWithInstruments(intervals[len(intervals)-1].End))
	zapLogger.Infof("Profit: %v", portfolio.GetProfit(intervals[len(intervals)-1].End))
	zapLogger.Infof("Remaining portfolio: %v", portfolio.GetInstruments())

	printInfo(tradingBot.GetInfo())

	<-ctx.Done()
	zapLogger.Infoln("start graceful shutdown")
}

func printInfo(info []backtest.IntervalProfit) {
	for _, i := range info {
		fmt.Printf("%f,", i.Balance)
	}
	fmt.Println()
	for _, i := range info {
		fmt.Printf("%f,", i.Profit)
	}
	fmt.Println()
	for _, i := range info {
		fmt.Printf("%s,", i.Ts)
	}
	fmt.Println()
}
