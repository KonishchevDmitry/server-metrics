package kernel

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

type errorMetrics struct {
	metric  *prometheus.CounterVec
	unknown prometheus.Counter
}

func newErrorMetrics() *errorMetrics {
	metric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: "kernel",
		Name:      "errors",
		Help:      "Count of kernel errors in /dev/kmsg.",
	}, []string{"type"})

	return &errorMetrics{
		metric:  metric,
		unknown: metric.WithLabelValues("unknown"),
	}
}
