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
	"github.com/STTM-NSU/trading-bot/internal/model"
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
// конфиг сттм
// lots balance strat
// orders config используем как коэфициенты для цен продажи и покупки, только маркет или лимит, имитируем покупку и отсутствие её в течение дня (24 часа)
// технические индикаторы дёргаю с апи и смотрю, мб пора раньще продать, только для этого
// учитываем комиссию для некотор

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

	// собрать стартовый портфель на стартовую сумму - не надо, дождёмся пятницы
	portfolio := backtest.NewPortfolio(zapLogger, cfg.StartAmountOfMoney[0].Value)

	candlesService := md.NewCandlesService(investClient, db, zapLogger)
	instrumentsService := instrument.NewInstrumentsService(investClient, zapLogger)
	techAnService := techan.NewTechAnalyseService(investClient, cfg.TechnicalIndicators, zapLogger)
	sttmService := sttm.NewSTTMService(cfg.STTM, zapLogger)
	executor := backtest.NewExecutor(zapLogger, model.InvestorTaxes, candlesService, portfolio)

	tradingBot := backtest.NewTradingBot(zapLogger, instrumentsService, cfg.Instruments, candlesService, techAnService, sttmService, executor, cfg.Orders, portfolio)
	intervals := backtest.SplitIntoWeeks(cfg.From.UTC(), cfg.To.UTC())

	intervalsProfits := make([]IntervalProfit, 0)

	for i, interval := range intervals { // iterate over weeks
		zapLogger.Infof("Interval: %v", interval)
		zapLogger.Infof("Balance: %v", portfolio.GetBalance())
		zapLogger.Infof("Profit: %v", portfolio.GetProfit())
		intervalsProfits = append(intervalsProfits, IntervalProfit{
			Balance: portfolio.GetBalance(),
			Profit:  portfolio.GetProfit(),
			Ts:      interval.Start,
		})
		for _, h := range backtest.DivideIntoHours(interval.Start, interval.End) {
			if h.Weekday() == time.Saturday || h.Weekday() == time.Sunday {
				continue
			}

			zapLogger.Debugf("Hour: %v", h)

			if h.Weekday() == time.Thursday && h.Hour() == 0 {
				tradingBot.SellOutRemaining()
				if i == len(intervals)-1 {
					tradingBot.SellOutPortfolio()
				}
			}

			if h.Weekday() == time.Friday && h.Hour() == 20 && i != len(intervals)-1 {
				zapLogger.Infof("Rebalance on: %s", h)
				if err := tradingBot.Rebalance(ctx, interval.Start, h); err != nil {
					zapLogger.Errorf("%s: rebalance failed", err)
				}
				continue
			}

			if h.Hour() == 0 {
				tradingBot.CheckTechIndicators(h)
			}
			tradingBot.ExecutorCheck(h)
		}
		if ctx.Err() != nil {
			break
		}
		// следить типо лимитными заявками за ценами
		// следить за имеющимся портфелем с помощью тех анализа
		// в пятницу запускаем ребаланс смотрим на цену конечную для пятницы и первую субботы
		// ребаланс даёт список инструментов которые: нужно продать выгодно, просто продать и купить
		// в этот момент относительно конфига считаем прибыль для проданных инструментов (разницу с ценой покупки и продажи)
		// на полученную итоговую сумму набираем новый портфель
	}

	zapLogger.Infof("All trading bot finished")
	zapLogger.Infof("Balance: %v", portfolio.GetBalance())
	zapLogger.Infof("Profit: %v", portfolio.GetProfit())
	zapLogger.Infof("Remaining portfolio: %v", portfolio.GetInstruments())
	intervalsProfits = append(intervalsProfits, IntervalProfit{
		Balance: portfolio.GetBalance(),
		Profit:  portfolio.GetProfit(),
		Ts:      intervals[len(intervals)-1].End,
	})

	for _, b := range intervalsProfits {
		fmt.Printf("%f,", b.Balance)
	}
	fmt.Println()
	for _, b := range intervalsProfits {
		fmt.Printf("%f,", b.Profit)
	}
	fmt.Println()
	for _, b := range intervalsProfits {
		fmt.Printf("%s,", b.Ts)
	}
	fmt.Println()

	<-ctx.Done()
	zapLogger.Infoln("start graceful shutdown")
}

type IntervalProfit struct {
	Balance float64
	Profit  float64
	Ts      time.Time
}
