package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(production bool, level string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, err
	}

	var cfg zap.Config
	if production {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	return cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
}
