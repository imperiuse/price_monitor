package market

import (
	"context"

	jsoniter "github.com/json-iterator/go"

	"github.com/imperiuse/price_monitor/internal/market/mock_external_api"
)

type (
	Currency = string
	Price    = int32 // TODO can be changed to int64 or float64, need more info about task

	Market interface {
		GetActualPrice(context.Context, Currency) Price
	}

	market struct{}

	ResponsePrice struct {
		Amount int32 `json:"amount"`
	}
)

func New() Market {
	return &market{}
}

func (m *market) GetActualPrice(ctx context.Context, _ Currency) Price {
	response, err := mock_external_api.RealWorldExternalApi(ctx)
	if err != nil {
		// TODO need process every errors, need more info about task
	}

	var data ResponsePrice

	if err = jsoniter.Unmarshal([]byte(response), &data); err != nil {
		// TODO need process error, need more info about task
		// data.Amount = 0
	}

	return data.Amount
}
