package logging

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"

	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

const encoderName = "custom"

var errorsMetric = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: metrics.Namespace,
	Subsystem: "metrics",
	Name:      "errors",
	Help:      "Metrics collection errors.",
})

func init() {
	prometheus.MustRegister(errorsMetric)

	if err := zap.RegisterEncoder(encoderName, func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return newEncoder(zapcore.NewConsoleEncoder(config)), nil
	}); err != nil {
		panic(err)
	}
}

func Configure(develMode bool) (*zap.SugaredLogger, error) {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.TimeKey = ""
	encoderConfig.LevelKey = ""

	var loggerConfig zap.Config
	if develMode {
		loggerConfig = zap.NewDevelopmentConfig()
	} else {
		loggerConfig = zap.NewProductionConfig()
	}

	loggerConfig.DisableCaller = true
	loggerConfig.DisableStacktrace = true

	loggerConfig.Encoding = encoderName
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
	return context.WithValue(ctx, contextKey{}, logger)
}

type encoder struct {
	zapcore.Encoder
}

func newEncoder(impl zapcore.Encoder) zapcore.Encoder {
	return encoder{impl}
}

func (e encoder) Clone() zapcore.Encoder {
	return newEncoder(e.Encoder.Clone())
}

func (e encoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	if entry.Level >= zapcore.WarnLevel {
		errorsMetric.Inc()
	}
	return e.Encoder.EncodeEntry(entry, fields)
}
