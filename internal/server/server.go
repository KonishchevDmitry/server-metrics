package server

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"

	"golang.org/x/sync/semaphore"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Start(logger *zap.SugaredLogger, collect func(ctx context.Context)) error {
	lock := semaphore.NewWeighted(1)
	prometheusHandler := promhttp.Handler()

	http.HandleFunc("/metrics", func(writer http.ResponseWriter, request *http.Request) {
		ctx := logging.WithLogger(request.Context(), logger)

		if lock.Acquire(ctx, 1) != nil {
			return
		}
		func() {
			defer lock.Release(1)
			collect(ctx)
		}()

		prometheusHandler.ServeHTTP(writer, request)
	})

	return http.ListenAndServe(":9101", nil)
}
