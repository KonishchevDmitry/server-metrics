package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sanity-io/litter"
	"go.uber.org/zap"
)

func Start(ctx context.Context, bindAddress string) error {
	http.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorLog:            prometheusLogger{logger: logging.L(ctx)},
		MaxRequestsInFlight: 2,
	}))

	server := http.Server{
		Addr:         bindAddress,
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
	logger, dump := l.logger.Errorf, true

	for _, value := range v {
		if err, ok := value.(error); ok {
			var netErr *net.OpError
			if errors.As(err, &netErr) && netErr.Op == "write" &&
				(netErr.Timeout() || errors.Is(netErr.Err, syscall.EPIPE)) || errors.Is(err, context.Canceled) {
				logger, dump = l.logger.Infof, false
			}
			break
		}
	}

	message := strings.TrimRight(fmt.Sprintf("%v", v), "\n")
	if dump {
		message = fmt.Sprintf("%s\n%s", message, litter.Sdump(v...))
	}

	logger("Prometheus: %s", message)
}
