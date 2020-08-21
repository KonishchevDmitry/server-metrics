package logging

import (
	"context"

	"go.uber.org/zap"
)

func Configure() (*zap.SugaredLogger, error) {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.TimeKey = ""
	encoderConfig.LevelKey = ""
	encoderConfig.CallerKey = ""

	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.EncoderConfig = encoderConfig

	logger, err := loggerConfig.Build()
	if err != nil {
		return nil, err
	}

	return logger.Sugar(), nil
}

type contextKey struct{}

func L(ctx context.Context) *zap.SugaredLogger {
	return ctx.Value(contextKey{}).(*zap.SugaredLogger)
}

func WithLogger(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(context.Background(), contextKey{}, logger)
}
