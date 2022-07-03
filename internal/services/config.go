package services

import (
	"go.uber.org/fx"

	"github.com/imperiuse/price_monitor/internal/services/controllers"
	"github.com/imperiuse/price_monitor/internal/services/storage"
)

// Config - config storage.
type Config struct {
	fx.Out

	Controllers controllers.Config `yaml:"controllers"`
	Storage     storage.Config     `yaml:"storage"`
}
