// Package utils provides shared utilities: structured logging, configuration helpers.
package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger builds a production-ready zap logger at the requested level.
// Valid levels: debug, info, warn, error. Unknown levels default to info.
func NewLogger(level string) *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		// Default to info on invalid level — fail-safe, never silence logs accidentally.
		zapLevel = zapcore.InfoLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	logger, err := cfg.Build()
	if err != nil {
		// If logger construction itself fails, panic — we cannot run safely without logging.
		panic("failed to initialize logger: " + err.Error())
	}
	return logger
}
