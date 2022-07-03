// Package scanner - package for price scanning
package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
	"github.com/imperiuse/price_monitor/internal/services/controllers"
	"github.com/imperiuse/price_monitor/internal/services/market"
	"github.com/imperiuse/price_monitor/internal/services/storage"
	"github.com/imperiuse/price_monitor/internal/services/storage/model"
)

type (
	// config - config of scanner Controller.
	config struct {
		cntWorkers            int
		timeoutOneTaskProcess time.Duration
		intervalPeriodicScan  time.Duration
	}

	Currency = model.CurrencyCode

	taskChan = chan Currency

	// ControllerDaemon - scanner controller.
	ControllerDaemon struct {
		*controllers.Base

		config  config
		storage storage.Storage
		market  market.Market

		taskCh            taskChan
		cancelWorkersFunc context.CancelFunc
	}
)

const name = "price_scanner"

// New - constructor of scanner ControllerDaemon.
func New(
	cfg controllers.Config,
	l *logger.Logger,
	s storage.Storage,
	m market.Market,
) (*ControllerDaemon, error) {
	c := &ControllerDaemon{
		Base:              controllers.New(name, l),
		config:            config{},
		storage:           s,
		market:            m,
		cancelWorkersFunc: func() {},
	}

	c.Base.RegisterShutdownFunc(
		func(ctx context.Context) { c.Shutdown(ctx) },
	)

	if err := c.parseConfig(cfg); err != nil {
		return nil, fmt.Errorf("scanner: c.parseConfig(cfg): %w", err)
	}

	c.taskCh = make(taskChan, c.config.cntWorkers)

	return c, nil
}

func (c *ControllerDaemon) parseConfig(cfg controllers.Config) error {
	var err error

	c.config.cntWorkers = cfg.Master.Scanner.CntWorkers

	c.config.timeoutOneTaskProcess, err = time.ParseDuration(cfg.Master.Scanner.TimeoutOneTaskProcess)
	if err != nil {
		return fmt.Errorf("%s: can't parse time.ParseDuration(c.config.TimeoutProcessOneRC): %w", c.Name, err)
	}

	c.config.intervalPeriodicScan, err = time.ParseDuration(cfg.Master.Scanner.IntervalPeriodicScan)
	if err != nil {
		return fmt.Errorf("%s: can't parse time.ParseDuration(c.config.IntervalPeriodicCheckStatusRC): %w",
			c.Name, err)
	}

	return nil
}

// Run - run controller func.
func (c *ControllerDaemon) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancelWorkersFunc = cancel

	go func(ctx context.Context) {
		c.Log.Info("[Scanner] Run")
		defer c.Log.Info("[Scanner] Finished")

		t := time.NewTicker(c.config.intervalPeriodicScan)
		defer t.Stop()

		for i := 0; i < c.config.cntWorkers; i++ {
			go c.scanWorker(ctx, i)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.prepareTasks(ctx)
			}
		}
	}(ctx)

	return nil
}

// prepareTasks - send task to scan direct RC to tasks chan.
func (c *ControllerDaemon) prepareTasks(ctx context.Context) {
	c.Log.Debug("[Scanner] prepareTasks start")

	// nolint rangeValCopy
	for _, v := range []Currency{model.BtcUsd} {
		select {
		case c.taskCh <- v:

		case <-ctx.Done():
			c.Log.Warn("[Scanner] prepareTasks ctx.Done")

			return
		}
	}

	c.Log.Debug("[Scanner] prepareTasks finished")
}

// Shutdown - shutdown func.
func (c *ControllerDaemon) Shutdown(_ context.Context) {
	c.cancelWorkersFunc()
}

func (c *ControllerDaemon) scanWorker(ctx context.Context, workerID int) {
	c.Log.Info("[ScanWorker] started", field.Int("workerID", workerID))
	defer c.Log.Info("[ScanWorker] finished", field.Int("workerID", workerID))

	for {
		select {
		case <-ctx.Done():
			return

		case currency, ok := <-c.taskCh:
			if !ok {
				c.Log.Error("[ScanWorker] received bad value from c.taskCh")
			}

			if err := c.processTask(ctx, currency); err != nil {
				c.Log.Error("err while process task", field.Int("workerID", workerID), field.Error(err))
			}
		}
	}
}

func (c *ControllerDaemon) processTask(ctx context.Context, currency Currency) error {
	const one = 1

	t, price, err := c.market.GetActualPrice(ctx, currency)
	if err != nil {
		return err
	}

	cnt, err := c.storage.Connector().RepoByName(model.PriceTableNameGetterFunc(currency)).
		Insert(ctx, []string{"time", "price"}, []any{t.Round(1000 * time.Millisecond), price})
	if err != nil {
		return err
	}
	if cnt != one {
		return storage.ErrNotInserted
	}

	return nil
}
