package executor

import (
	"fmt"
	"time"

	"github.com/STTM-NSU/trading-bot/internal/config"
	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
	"github.com/STTM-NSU/trading-bot/internal/tools"
	"github.com/google/uuid"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	investapi "github.com/russianinvestments/invest-api-go-sdk/proto"
	"go.uber.org/ratelimit"
)

const (
	_orderIdPrefix = "sttm-trading-bot"
)

type Executor struct {
	cfg    config.OrdersConfig
	logger logger.Logger

	stopOrdersRateLimiter ratelimit.Limiter // 50 T/M
	ordersRateLimiter     ratelimit.Limiter // 100 T/M

	stopOrdersService *investgo.StopOrdersServiceClient
	ordersService     *investgo.OrdersServiceClient
}

func NewExecutor(c *investgo.Client, cfg config.OrdersConfig, logger logger.Logger) *Executor {
	return &Executor{
		stopOrdersService:     c.NewStopOrdersServiceClient(),
		ordersService:         c.NewOrdersServiceClient(),
		stopOrdersRateLimiter: ratelimit.New(50, ratelimit.Per(time.Minute)),
		ordersRateLimiter:     ratelimit.New(100, ratelimit.Per(time.Minute)),
		cfg:                   cfg,
		logger:                logger,
	}
}

func (e *Executor) Buy(price float64, i model.PortfolioInstrument) (string, string, error) {
	var (
		orderRequestId, orderId string
		err                     error
	)
	switch e.cfg.BuyOrder.Type {
	case config.TakeProfit, config.StopLoss, config.StopLimit:
		orderRequestId, orderId, err = e.buyStop(price, i, e.cfg.BuyOrder)
		if err != nil {
			return "", "", fmt.Errorf("%w: buyStop err", err)
		}
	case config.Market, config.Limit:
		orderRequestId, orderId, err = e.buyMarket(price, i, e.cfg.BuyOrder)
		if err != nil {
			return "", "", fmt.Errorf("%w: buyMarket err", err)
		}
	}

	return orderRequestId, orderId, err
}

func (e *Executor) buyMarket(price float64, i model.PortfolioInstrument, cfg config.OrderConfig) (string, string, error) {
	orderRequestId := _orderIdPrefix + uuid.NewString()

	e.ordersRateLimiter.Take()
	resp, err := e.ordersService.Buy(&investgo.PostOrderRequestShort{
		InstrumentId: i.InstrumentID,
		Quantity:     int64(i.Quantity),
		Price:        tools.FloatToQuotation(price*(1-cfg.ProfitPercentIndent), i.MinPriceIncrement), // limit order price
		AccountId:    i.AccountID,
		OrderType:    cfg.Type.ToInvestType(),
		OrderId:      orderRequestId,
	})
	if err != nil {
		return "", "", fmt.Errorf("%w: can't buy market", err)
	}

	return orderRequestId, resp.GetOrderId(), nil
}

func (e *Executor) buyStop(price float64, i model.PortfolioInstrument, cfg config.OrderConfig) (string, string, error) {
	orderRequestId := _orderIdPrefix + uuid.NewString()
	req := &investgo.PostStopOrderRequest{
		InstrumentId:   i.InstrumentID,
		Quantity:       int64(i.Quantity),
		Direction:      investapi.StopOrderDirection_STOP_ORDER_DIRECTION_BUY,
		AccountId:      i.AccountID,
		ExpirationType: investapi.StopOrderExpirationType_STOP_ORDER_EXPIRATION_TYPE_GOOD_TILL_CANCEL,
		StopOrderType:  cfg.Type.ToInvestStopType(),
		PriceType:      investapi.PriceType_PRICE_TYPE_CURRENCY,
		OrderID:        orderRequestId,
	}

	switch cfg.Type {
	case config.StopLoss:
		req.StopPrice = tools.FloatToQuotation(price*(1+cfg.ProfitPercentIndent), i.MinPriceIncrement)
	case config.StopLimit:
		req.Price = tools.FloatToQuotation(price*(1+cfg.DefencePercentIndent), i.MinPriceIncrement)
		req.StopPrice = tools.FloatToQuotation(price*(1+cfg.ProfitPercentIndent), i.MinPriceIncrement)
	case config.TakeProfit:
		req.StopPrice = tools.FloatToQuotation(price*(1-cfg.ProfitPercentIndent), i.MinPriceIncrement)
		req.ExchangeOrderType = investapi.ExchangeOrderType_EXCHANGE_ORDER_TYPE_MARKET
		req.TakeProfitType = investapi.TakeProfitType_TAKE_PROFIT_TYPE_REGULAR
	}

	e.stopOrdersRateLimiter.Take()
	resp, err := e.stopOrdersService.PostStopOrder(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: can't buy stop", err)
	}

	return orderRequestId, resp.GetStopOrderId(), nil
}

func (e *Executor) SellOutProfit(price float64, i model.PortfolioInstrument) (string, string, error) {
	return e.Sell(price, i, e.cfg.SellOutProfit)
}

func (e *Executor) SellOrder(price float64, i model.PortfolioInstrument) (string, string, error) {
	return e.Sell(price, i, e.cfg.SellOrder)
}

func (e *Executor) SellHedge(price float64, i model.PortfolioInstrument) (string, string, error) {
	return e.Sell(price, i, e.cfg.HedgeOrder)
}

func (e *Executor) Sell(price float64, i model.PortfolioInstrument, cfg config.OrderConfig) (string, string, error) {
	var (
		orderRequestId, orderId string
		err                     error
	)
	switch cfg.Type {
	case config.TakeProfit, config.StopLoss, config.StopLimit:
		orderRequestId, orderId, err = e.sellStop(price, i, cfg)
		if err != nil {
			return "", "", fmt.Errorf("%w: sellStop err", err)
		}
	case config.Market, config.Limit:
		orderRequestId, orderId, err = e.sellMarket(price, i, cfg)
		if err != nil {
			return "", "", fmt.Errorf("%w: sellMarket err", err)
		}
	}

	return orderRequestId, orderId, err
}

func (e *Executor) sellMarket(price float64, i model.PortfolioInstrument, cfg config.OrderConfig) (string, string, error) {
	orderRequestId := _orderIdPrefix + uuid.NewString()

	e.ordersRateLimiter.Take()
	resp, err := e.ordersService.Sell(&investgo.PostOrderRequestShort{
		InstrumentId: i.InstrumentID,
		Quantity:     int64(i.Quantity),
		Price:        tools.FloatToQuotation(price*(1+cfg.ProfitPercentIndent), i.MinPriceIncrement), // limit order price
		AccountId:    i.AccountID,
		OrderType:    cfg.Type.ToInvestType(),
		OrderId:      orderRequestId,
	})
	if err != nil {
		return "", "", fmt.Errorf("%w: can't sell market", err)
	}

	return orderRequestId, resp.GetOrderId(), nil
}

func (e *Executor) sellStop(price float64, i model.PortfolioInstrument, cfg config.OrderConfig) (string, string, error) {
	orderRequestId := _orderIdPrefix + uuid.NewString()
	req := &investgo.PostStopOrderRequest{
		InstrumentId:   i.InstrumentID,
		Quantity:       int64(i.Quantity),
		Direction:      investapi.StopOrderDirection_STOP_ORDER_DIRECTION_SELL,
		AccountId:      i.AccountID,
		ExpirationType: investapi.StopOrderExpirationType_STOP_ORDER_EXPIRATION_TYPE_GOOD_TILL_CANCEL,
		StopOrderType:  cfg.Type.ToInvestStopType(),
		PriceType:      investapi.PriceType_PRICE_TYPE_CURRENCY,
		OrderID:        orderRequestId,
	}

	switch cfg.Type {
	case config.StopLoss:
		req.StopPrice = tools.FloatToQuotation(price*(1-cfg.ProfitPercentIndent), i.MinPriceIncrement)
	case config.StopLimit:
		req.Price = tools.FloatToQuotation(price*(1-cfg.DefencePercentIndent), i.MinPriceIncrement)
		req.StopPrice = tools.FloatToQuotation(price*(1-cfg.ProfitPercentIndent), i.MinPriceIncrement)
	case config.TakeProfit:
		req.StopPrice = tools.FloatToQuotation(price*(1+cfg.ProfitPercentIndent), i.MinPriceIncrement)
		req.ExchangeOrderType = investapi.ExchangeOrderType_EXCHANGE_ORDER_TYPE_MARKET
		req.TakeProfitType = investapi.TakeProfitType_TAKE_PROFIT_TYPE_REGULAR
	}

	e.stopOrdersRateLimiter.Take()
	resp, err := e.stopOrdersService.PostStopOrder(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: can't sell stop", err)
	}

	return orderRequestId, resp.GetStopOrderId(), nil
}

func (e *Executor) CancelOrder(i model.PortfolioInstrument, t config.OrderType) error {
	switch t {
	case config.StopLimit, config.StopLoss, config.TakeProfit:
		return e.cancelStopOrder(i)
	case config.Market, config.Limit:
		return e.cancelMarketOrder(i)
	}
	return fmt.Errorf("unknown order type: %s", t)
}

func (e *Executor) cancelStopOrder(i model.PortfolioInstrument) error {
	e.stopOrdersRateLimiter.Take()
	_, err := e.stopOrdersService.CancelStopOrder(i.AccountID, i.OrderID)
	if err != nil {
		return fmt.Errorf("%w: can't cancel stop order", err)
	}
	return nil
}

func (e *Executor) cancelMarketOrder(i model.PortfolioInstrument) error {
	e.ordersRateLimiter.Take()
	var (
		orderIdType = new(investapi.OrderIdType)
		id          string
	)
	if i.OrderID != "" {
		*orderIdType = investapi.OrderIdType_ORDER_ID_TYPE_EXCHANGE
		id = i.OrderID
	} else if i.OrderRequestID != "" {
		*orderIdType = investapi.OrderIdType_ORDER_ID_TYPE_REQUEST
		id = i.OrderRequestID
	} else {
		return fmt.Errorf("empty orders id")
	}

	_, err := e.ordersService.CancelOrder(i.AccountID, id, orderIdType)
	if err != nil {
		return fmt.Errorf("%w: can't cancel order", err)
	}
	return nil
}
