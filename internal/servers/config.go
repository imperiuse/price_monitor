package servers

import (
	"go.uber.org/fx"

	"github.com/imperiuse/price_monitor/internal/servers/http"
)

// Config - config.
type Config struct {
	fx.Out

	HTTP http.Config
}
