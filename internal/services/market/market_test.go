package market

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	assert.NotNil(t, New())
}

func TestGetActualPrice(t *testing.T) {
	m := New()

	for i := 0; i < 100; i++ {
		tt, price, err := m.GetActualPrice(context.Background(), "btcusdt")
		assert.Nil(t, err)
		assert.NotNil(t, tt)
		assert.True(t, price > 39000 && price < 41000)
	}
}
