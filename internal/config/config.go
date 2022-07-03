package config

import (
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/config"
	"go.uber.org/fx"
	"path"

	"github.com/imperiuse/price_monitor/internal/consul"
	"github.com/imperiuse/price_monitor/internal/env"
	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/servers"
	"github.com/imperiuse/price_monitor/internal/services"
)

//nolint gosec golint
const (
	EnvNamePostgresUser     = "PM_POSTGRES_USER"
	EnvNamePostgresPassword = "PM_POSTGRES_PASSWORD"
)

// Config - main config.
type Config struct {
	fx.Out

	Logger logger.Config
	Consul consul.Config

	Services services.Config
	Servers  servers.Config
}

// New - create new global config.
func New(appName string, envName env.Var, configPath string, nodeName string) (Config, error) {
	y, err := config.NewYAML(
		config.File(path.Join(configPath, "default.yml")),
		config.File(path.Join(configPath, envName+".yml")),
	)
	if err != nil {
		return Config{}, fmt.Errorf("config.NewYAML: %w", err)
	}

	cfg := Config{}
	err = y.Get("").Populate(&cfg)

	if err != nil {
		return cfg, fmt.Errorf("cfg.Populate: %w", err)
	}

	applyEnvOnConfig(&cfg, appName)

	cfg.Servers.HTTP.Name += "_" + nodeName

	// show config for debug purposes
	// nolint forbidigo // exception of rule )
	fmt.Printf("NodeName: %s, AppName: %s; EnvName: %s; Config: %+v", nodeName, appName, envName, cfg)

	return cfg, nil
}

func applyEnvOnConfig(cfg *Config, appName string) {
	v := viper.New()
	v.AutomaticEnv()

	cfg.Services.Storage.Username = v.GetString(EnvNamePostgresUser)
	cfg.Services.Storage.Password = v.GetString(EnvNamePostgresPassword)
}
