package timescaledb

import "github.com/imperiuse/price_monitor/internal/storage"

type store struct {
}

func New() storage.Storage {
	return &store{}
}

func (_ store) Connect() {}

func (_ store) Close() {}
