package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/joho/godotenv"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

const (
	_investCfgFilePath = "./configs/invest.yaml"
)

func main() {
	zapLogger, loggerSync, err := logger.NewZapLogger(logger.Debug)
	if err != nil {
		log.Fatalf("%s: can't init logger", err)
	}
	defer loggerSync()

	if err := godotenv.Load(); err != nil {
		zapLogger.Warnf("can't detect .env file")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	investCfg, err := config.LoadInvestConfig(_investCfgFilePath)
	if err != nil {
		zapLogger.Fatalf("%s: can't load invest cfg", err)
	}

	investClient, err := investgo.NewClient(ctx, investCfg, zapLogger)
	if err != nil {
		zapLogger.Fatalf("%s: can't create invest client", err)
	}

	s := investClient.NewUsersServiceClient()
	i, err := s.GetInfo()
	if err != nil {
		zapLogger.Fatalf("%s: can't get info", err)
	}
	zapLogger.Infoln(i.GetTariff())
}
