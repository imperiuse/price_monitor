package controllers

import (
	"context"
	"time"

	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
)

// DaemonController - interface which describe DaemonController service.
type DaemonController interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context)
}

type (

	// Base - base daemon controller ("parent" for all controllers).
	Base struct {
		Name string
		Log  *logger.Logger

		handlers     []Handler
		shutdownFunc ShutdownFunc
	}

	// Handler - handlers for broker events.
	Handler = func(ctx context.Context) error
	// ShutdownFunc - func which will invoke when controller will finished.
	ShutdownFunc = func(ctx context.Context)
)

// New - constructor of Base controller.
func New(name string, log *logger.Logger) *Base {
	return &Base{
		Name:         name,
		Log:          log.With(field.Controller(name)),
		handlers:     nil,                          // no handlers. Yes, it safety for usage in range loop
		shutdownFunc: func(ctx context.Context) {}, // do nothing
	}
}

// RegisterHandlers - register [] Handler.
func (b *Base) RegisterHandlers(h ...Handler) {
	b.handlers = append(b.handlers, h...)
}

// RegisterShutdownFunc - register ShutdownFunc.
func (b *Base) RegisterShutdownFunc(f ShutdownFunc) {
	b.shutdownFunc = f
}

// Run - default runner for controllers.
func (b *Base) Run(ctx context.Context) error {
	for _, h := range b.handlers {
		err := h(ctx)
		if err != nil {
			return err
		}
	}

	// nolint, that's ok, ctx is already Done, it's dead, need new ctx
	go func(ctx context.Context) {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// nolint
		b.Shutdown(shutdownCtx)
	}(ctx)

	return nil
}

// Shutdown - runs shutdown func register for direct controller.
func (b *Base) Shutdown(ctx context.Context) {
	b.shutdownFunc(ctx)
}
