package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/imperiuse/price_monitor/internal/env"
)

type (
	// Config - logger config.
	Config struct {
		Level    string
		Encoding string
		Color    bool
		Outputs  []string
		Tags     []string
	}

	// Logger logger wrapper under zap.Logger.
	Logger = zap.Logger
)

// NewNop - new Nop Logger.
func NewNop() *Logger {
	return zap.NewNop()
}

// New - create new logger.
func New(config Config, e, serviceName, version string) (*Logger, error) {
	cfg := zap.NewProductionConfig()
	var lvl zapcore.Level

	err := lvl.UnmarshalText([]byte(config.Level))
	if err != nil {
		return nil, fmt.Errorf("lvl.UnmarshalText: %w", err)
	}

	cfg.Level.SetLevel(lvl)
	cfg.DisableStacktrace = true
	cfg.Development = e == env.Dev
	cfg.Sampling.Initial = 50
	cfg.Sampling.Thereafter = 50
	cfg.Encoding = config.Encoding
	cfg.OutputPaths = config.Outputs
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	if config.Color {
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("cfg.Build: %w", err)
	}

	return logger.With(
			zap.String("env", e),
			zap.String("version", version),
			zap.String("services", serviceName),
		),
		nil
}

// LogIfError - log only if err!=nil.
func LogIfError(l *Logger, msg string, err error, f ...zapcore.Field) {
	LogCustomIfError(l.Error, msg, err, f...)
}

// LogCustomIfError - log with custom lvl only if err!=nil.
func LogCustomIfError(logFunc func(string, ...zapcore.Field), msg string, err error, f ...zapcore.Field) {
	if err != nil {
		logFunc(msg, append(f, zap.Error(err))...)
	}
}
