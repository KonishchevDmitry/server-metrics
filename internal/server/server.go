package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Start(ctx context.Context) error {
	http.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorLog:            prometheusLogger{logger: logging.L(ctx)},
		DisableCompression:  true,
		MaxRequestsInFlight: 2,
	}))

	server := http.Server{
		Addr:         "127.0.0.1:9101",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     log.New(httpLogger{logger: logging.L(ctx)}, "", 0),
	}

	return server.ListenAndServe()
}

type httpLogger struct {
	logger *zap.SugaredLogger
}

func (l httpLogger) Write(data []byte) (n int, err error) {
	size := len(data)
	if size != 0 && data[size-1] == '\n' {
		data = data[:size-1]
	}

	l.logger.Errorf("HTTP server: %s", data)
	return size, nil
}

type prometheusLogger struct {
	logger *zap.SugaredLogger
}

func (l prometheusLogger) Println(v ...interface{}) {
	l.logger.Errorf("Prometheus: %s.", fmt.Sprintln(v...))
}
