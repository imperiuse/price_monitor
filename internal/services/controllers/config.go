package controllers

import (
	"github.com/imperiuse/price_monitor/internal/services/controllers/general"
	"github.com/imperiuse/price_monitor/internal/services/controllers/master"
	"github.com/imperiuse/price_monitor/internal/services/controllers/slave"
)

// Config - config of all controllers.
type Config struct {
	Slave   slave.Config
	General general.Config
	Master  master.Config
}
