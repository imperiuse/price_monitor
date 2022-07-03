package market

import (
	"context"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/imperiuse/price_monitor/internal/services/market/mock_external_api"
)

type (
	Currency = string
	Price    = float64 // TODO can be changed to int64 ordecimal or other need more context

	Market interface {
		GetActualPrice(context.Context, Currency) (time.Time, Price, error)
	}

	market struct{}

	ResponsePrice struct {
		Amount Price `json:"amount"`
	}
)

func New() Market {
	return &market{}
}

func (m *market) GetActualPrice(ctx context.Context, _ Currency) (time.Time, Price, error) {
	t, response, err := mock_external_api.RealWorldExternalApi(ctx)
	if err != nil {
		// TODO need process every errors, need more info about task

		return t, 0, fmt.Errorf("problem get data from external api: %w", err)
	}

	var data ResponsePrice

	if err = jsoniter.Unmarshal([]byte(response), &data); err != nil {
		// TODO need process error, need more info about task

		return t, 0, fmt.Errorf("problem to unmarshal response from external api: %w", err)
	}

	return t, data.Amount, nil
}
