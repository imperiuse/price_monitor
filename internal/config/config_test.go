package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/imperiuse/price_monitor/internal/env"
)

func Test_New(t *testing.T) {
	cfg, err := New("core", env.Dev, "../../configs", "")
	assert.NotNil(t, cfg)
	assert.Nil(t, err)

	cfg, err = New("core", env.Prod, "../../configs", "")
	assert.NotNil(t, cfg)
	assert.Nil(t, err)

	cfg, err = New("core", env.Test, "../../configs", "")
	assert.NotNil(t, cfg)
	assert.Nil(t, err)
}
