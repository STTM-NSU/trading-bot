package position

import (
	"context"
	"sync"

	"github.com/STTM-NSU/trading-bot/internal/logger"
	"github.com/STTM-NSU/trading-bot/internal/model"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

type PositionsService struct {
	opsClient       *investgo.OperationsServiceClient
	opsStreamClient *investgo.OperationsStreamClient

	accountID string
	logger    logger.Logger
}

func NewPositionsService(c *investgo.Client, accountID string, logger logger.Logger) *PositionsService {
	return &PositionsService{
		opsClient:       c.NewOperationsServiceClient(),
		opsStreamClient: c.NewOperationsStreamClient(),
		accountID:       accountID,
		logger:          logger,
	}
}

func (s *PositionsService) ListenPositions(ctx context.Context) (<-chan []model.Balance, error) {
	stream, err := s.opsStreamClient.PositionsStream([]string{s.accountID})
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	ch := make(chan []model.Balance, 10)
	defer func() {
		wg.Wait()
		close(ch)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := stream.Listen(); err != nil {
			s.logger.Errorf("error listening positions stream: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				stream.Stop()
				return
			case v, ok := <-stream.Positions():
				if !ok {
					stream.Stop()
					return
				}
				if len(v.GetMoney()) == 0 {
					continue
				}

				b := make([]model.Balance, 0, len(v.GetMoney()))
				for _, m := range v.GetMoney() {
					b = append(b, model.Balance{
						Value:     m.GetAvailableValue().ToFloat(),
						Currency:  m.GetAvailableValue().GetCurrency(),
						AccountID: s.accountID,
					})
				}

				ch <- b
			}
		}
	}()

	return ch, nil
}

func (s *PositionsService) UnaryGetPositions() ([]model.Balance, error) {
	resp, err := s.opsClient.GetPositions(s.accountID)
	if err != nil {
		return nil, err
	}

	if len(resp.GetMoney()) == 0 {
		return nil, nil
	}

	b := make([]model.Balance, 0, len(resp.GetMoney()))
	for _, m := range resp.GetMoney() {
		b = append(b, model.Balance{
			Value:     m.ToFloat(),
			Currency:  m.GetCurrency(),
			AccountID: s.accountID,
		})
	}

	return b, nil
}
