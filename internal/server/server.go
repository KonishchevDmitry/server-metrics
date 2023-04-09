package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func Start(ctx context.Context) error {
	http.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorLog:            &prometheusLogger{},
		DisableCompression:  true,
		MaxRequestsInFlight: 2,
	}))

	server := http.Server{
		Addr:         "127.0.0.1:9101",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}

type prometheusLogger struct {
	logger *zap.SugaredLogger
}

func (l *prometheusLogger) Println(v ...interface{}) {
	l.logger.Errorf("Prometheus: %s.", fmt.Sprintln(v...))
}
