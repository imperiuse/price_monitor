package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/imperiuse/price_monitor/internal/consul"
	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
	"github.com/imperiuse/price_monitor/internal/services/controllers"
	"github.com/imperiuse/price_monitor/internal/services/storage"
)

const controllerMasterLockKey = "pm/services/controllers/master/master_lock_key"

type (

	// config - config for master-monitor controller.
	config struct {
		timeoutConsulLeaderCheck time.Duration
		shutdownCtxTimeout       time.Duration

		appVersion string
	}

	// Controller - struct which check is it app(node) master or not, also,
	// it run or shutdown all other master-controllers.
	Controller struct {
		*controllers.Base

		config  config
		consul  *consul.Client
		storage storage.Storage

		masterControllers []controllers.DaemonController

		isMaster              bool
		allControllersStarted bool
	}
)

const name = "monitor"

//nolint exported
// New - constructor of monitor controller
func New(
	appVersion string,
	cfg controllers.Config,
	logger *logger.Logger,
	consul *consul.Client,
	storage storage.Storage,
	masterControllers ...controllers.DaemonController,
) (*Controller, error) {
	c := &Controller{
		Base:                  controllers.New(name, logger),
		config:                config{appVersion: appVersion},
		consul:                consul,
		storage:               storage,
		masterControllers:     masterControllers,
		isMaster:              false,
		allControllersStarted: false,
	}

	c.Base.RegisterShutdownFunc(
		func(ctx context.Context) { c.shutdownAllMasterControllers() },
	)

	return c, c.parseConfig(cfg)
}

// Run - start monitor controller.
func (c *Controller) Run(ctx context.Context) error {
	c.checkTrySetMasterFlag(ctx)

	go func(ctx context.Context) {
		c.Log.Info("[Monitor] Run")
		defer c.Log.Info("[Monitor] Finished")

		tConsulCoreLeader := time.NewTicker(c.config.timeoutConsulLeaderCheck)
		defer tConsulCoreLeader.Stop()

		for {
			select {
			case <-ctx.Done():
				c.Log.Warn("[Monitor] ctx.Done")

				// nolint
				c.shutdownAllMasterControllers()

				return

			case <-tConsulCoreLeader.C:
				c.checkTrySetMasterFlag(ctx)

				if c.isMaster && !c.allControllersStarted {
					// nolint
					c.shutdownAllMasterControllers()

					if err := c.consul.DestroySession(); err == nil {
						c.isMaster = false
					}
				}
			}
		}
	}(ctx)

	return nil
}

func (c *Controller) parseConfig(cfg controllers.Config) error {
	var err error

	c.config.timeoutConsulLeaderCheck, err = time.ParseDuration(cfg.General.Monitor.TimeoutConsulLeaderCheck)
	if err != nil {
		return fmt.Errorf("%s: can't parse cfg.General.Monitor.TimeoutConsulCoreLeaderCheck): %w", c.Name, err)
	}

	return nil
}

func (c *Controller) runAllMasterControllers(ctx context.Context) error {
	for _, controller := range c.masterControllers {
		err := controller.Run(ctx)
		if err != nil {
			return fmt.Errorf("[Monitor] runAllMasterControllers: %w", err)
		}
	}

	return nil
}

func (c *Controller) shutdownAllMasterControllers() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), c.config.shutdownCtxTimeout)
	defer cancel()

	for _, controller := range c.masterControllers {
		controller.Shutdown(shutdownCtx)
	}
}

func (c *Controller) checkTrySetMasterFlag(ctx context.Context) {
	c.Log.Debug("[Monitor] check leadership")

	newIsMaster, noOneIsMaster := c.consul.CheckLeadership(c.isMaster, controllerMasterLockKey)

	if noOneIsMaster {
		c.Log.Warn("[Monitor] try become a master")

		var err error
		newIsMaster, err = c.consul.TryBecomeLeader(controllerMasterLockKey)

		if err != nil {
			c.Log.Error("[Monitor] err while c.consul.TryBecomeLeader", field.Error(err))

			newIsMaster = false
		}
	}

	c.setMasterFlag(ctx, newIsMaster)
}

// nolint nestif
func (c *Controller) setMasterFlag(ctx context.Context, newIsMaster bool) {
	if newIsMaster != c.isMaster {
		if newIsMaster {
			c.allControllersStarted = true
			if err := c.runAllMasterControllers(ctx); err != nil {
				c.allControllersStarted = false
				c.Log.Error("[Monitor] runAllMasterControllers error", field.Error(err))
			}
		} else {
			c.Log.Warn("[Monitor] I have lost master flag")
			c.shutdownAllMasterControllers()
			c.allControllersStarted = false
		}
	} else {
		if newIsMaster { // условие newIsMaster эквивалентно newIsMaster == c.isMaster && c.isMaster
			c.Log.Debug("[Monitor] try renew session")
			if err := c.consul.RenewSession(); err != nil {
				c.Log.Error("[Monitor] error while c.consul.RenewSession()", field.Error(err))
			}
		}
	}

	c.isMaster = newIsMaster
}
