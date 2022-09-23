package server

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	model "github.com/prometheus/client_model/go"
	"golang.org/x/sync/semaphore"
)

func Start(ctx context.Context, collect func(ctx context.Context)) error {
	lock := semaphore.NewWeighted(1)
	gatherer := lockedGatherer{lock: lock}
	prometheusHandler := promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})

	http.HandleFunc("/metrics", func(writer http.ResponseWriter, request *http.Request) {
		if lock.Acquire(request.Context(), 1) != nil {
			return
		}
		collect(ctx)
		lock.Release(1)

		prometheusHandler.ServeHTTP(writer, request)
	})

	server := http.Server{
		Addr:         ":9101",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}

type lockedGatherer struct {
	lock *semaphore.Weighted
}

func (g lockedGatherer) Gather() ([]*model.MetricFamily, error) {
	if err := g.lock.Acquire(context.Background(), 1); err != nil {
		return nil, err
	}
	defer g.lock.Release(1)
	return prometheus.DefaultGatherer.Gather()
}
