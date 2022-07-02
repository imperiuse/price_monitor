package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/jackc/pgx/v4"        // for pgx driver import.
	_ "github.com/jackc/pgx/v4/stdlib" // for pgx driver import.
	_ "go.uber.org/automaxprocs"       // Automatically set GOMAXPROCS to match Linux container CPU quota.

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/imperiuse/price_monitor/internal/config"
	"github.com/imperiuse/price_monitor/internal/consul"
	"github.com/imperiuse/price_monitor/internal/env"
	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
	"github.com/imperiuse/price_monitor/internal/servers/http"
	"github.com/imperiuse/price_monitor/internal/storage"
	"github.com/imperiuse/price_monitor/internal/storage/timescaledb"
	"github.com/imperiuse/price_monitor/internal/uuid"
)

const (
	appStartTimeout = 10 * time.Second
	appStopTimeout  = 10 * time.Second

	shutDownTimeout = 5 * time.Second
)

// THIS VARS PASSED INTO PROGRAM BY -X arg in build step (@see Dockerfile).
// nolint gochecknoglobals
var (
	// AppName - имя приложения.
	AppName = env.Name
	// AppVersion - версия приложения ( CI_COMMIT_TAG ).
	AppVersion = "vX.X.X"
	// AppEnv - окружение (dev|test|stage|prod).
	AppEnv = env.Dev
)

var AppNodeName = uuid.MustUUID4()

// ConfigPath - config path.
var ConfigPath = flag.String("config", "./configs", "configs path")

type application struct {
	name       string
	version    string
	env        string
	configPath string

	startTimeout time.Duration
	stopTimeout  time.Duration
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.Parse()

	app := &application{
		name:    AppName,
		version: AppVersion,
		env:     AppEnv,

		configPath: *ConfigPath,

		startTimeout: appStartTimeout,
		stopTimeout:  appStopTimeout,
	}

	app.run()
}

// Use DI patteern -> https://en.wikipedia.org/wiki/Dependency_injection
func (a *application) run() {
	fxApp := fx.New(
		fx.Provide(
			func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			func() (config.Config, error) {
				cfg, err := config.New(a.name, a.env, a.configPath, AppNodeName)
				if err != nil {
					return cfg, fmt.Errorf("config.New: %w", err)
				}

				return cfg, nil
			},
			func(config logger.Config) (*logger.Logger, error) {
				return logger.New(config, a.env, a.name, a.version)
			},
			consul.New,
			timescaledb.New,
			func(cfg http.Config, l *logger.Logger, s storage.Storage) (*http.Server, error) {
				return http.New(a.env, cfg, l, s)
			},
		),
		fx.Invoke(a.start),
		fx.StartTimeout(a.startTimeout),
		fx.StopTimeout(a.startTimeout),
	)

	fxApp.Run()
}

func (a *application) start(
	globalContext context.Context,
	globalContextCancel context.CancelFunc,
	lc fx.Lifecycle,
	logger *logger.Logger,
	consul *consul.Client,
	storage storage.Storage,
	httpServer *http.Server,
) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			logger.Info("starting " + a.name)

			if err := consul.Register(logger, httpServer); err != nil {
				logger.Error("error on consul registration", zap.Error(err))

				return fmt.Errorf("problem consul.Register: %w", err)
			}

			logger.Info("Apply migration")
			// TODO use libs like goose or smth else  //	"github.com/golang-migrate/migrate/v4/source"

			httpServer.Run()

			// todo env related stuff
			switch a.env {
			case env.Dev:
			case env.Stage:
			case env.Test:
			case env.Prod:
			}

			return nil
		},
		OnStop: func(_ context.Context) error {
			shutDownCtx, shutDownCtxCancel := context.WithTimeout(context.Background(), shutDownTimeout)
			defer shutDownCtxCancel()

			consul.Deregister(logger, httpServer)

			httpServer.Stop(shutDownCtx)

			globalContextCancel()

			storage.Close()

			logger.Info("stopped", field.Error(logger.Sync()))

			return nil
		},
	})
}
