package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(production bool) (*zap.Logger, error) {
	var cfg zap.Config
	if production {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	return cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
}
