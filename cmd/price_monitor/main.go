package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/imperiuse/price_monitor/internal/services/storage/model"
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
	"github.com/imperiuse/price_monitor/internal/services/controllers"
	"github.com/imperiuse/price_monitor/internal/services/controllers/general/monitor"
	"github.com/imperiuse/price_monitor/internal/services/controllers/master/scanner"
	"github.com/imperiuse/price_monitor/internal/services/market"
	"github.com/imperiuse/price_monitor/internal/services/storage"
	"github.com/imperiuse/price_monitor/internal/services/storage/timescaledb"
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
			func(storageCfg storage.Config, logger *logger.Logger,
			) (storage.Storage, error) {
				return timescaledb.New(storageCfg, logger)
			},
			func(cfg http.Config, l *logger.Logger, s storage.Storage) (*http.Server, error) {
				return http.New(a.env, cfg, l, s)
			},
			market.New,
			scanner.New,
		),
		fx.Invoke(a.start),
		fx.StartTimeout(a.startTimeout),
		fx.StopTimeout(a.startTimeout),
	)

	fxApp.Run()
}

func (a *application) start(
	lc fx.Lifecycle,
	globalContext context.Context,
	globalContextCancel context.CancelFunc,
	controllersCfg controllers.Config,
	log *logger.Logger,
	consul *consul.Client,
	storage storage.Storage,
	httpServer *http.Server,
	scanner *scanner.ControllerDaemon,
) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("starting " + a.name)

			if err := consul.Register(log, httpServer); err != nil {
				log.Error("error on consul registration", zap.Error(err))

				return fmt.Errorf("problem consul.Register: %w", err)
			}

			log.Info("Apply migration")
			// TODO use libs like goose or smth else  //	"github.com/golang-migrate/migrate/v4/source"

			httpServer.Run()

			// todo env related stuff
			switch a.env {
			case env.Dev:
				logger.LogIfError(log, "Refresh error", storage.Refresh(globalContext, []model.Table{
					model.Monitoring{}.Repo(),
					model.PriceTableNameGetterFunc(model.BtcUsd),
				}),
				)
			case env.Stage:
			case env.Test:
			case env.Prod:
			}

			mon, err := monitor.New(a.version, controllersCfg, log, consul, storage,
				[]controllers.DaemonController{scanner}...)
			if err != nil {
				return fmt.Errorf("can't create monitor: %w", err)
			}

			return mon.Run(globalContext)
		},
		OnStop: func(_ context.Context) error {
			shutDownCtx, shutDownCtxCancel := context.WithTimeout(context.Background(), shutDownTimeout)
			defer shutDownCtxCancel()

			consul.Deregister(log, httpServer)

			httpServer.Stop(shutDownCtx)

			globalContextCancel()

			storage.Close()

			log.Info("stopped", field.Error(log.Sync()))

			return nil
		},
	})
}
