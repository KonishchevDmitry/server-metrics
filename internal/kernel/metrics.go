package kernel

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

func newErrorsMetric() *prometheus.CounterVec {
	metric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: "kernel",
		Name:      "errors",
		Help:      "Count of kernel errors in /dev/kmsg.",
	}, []string{"type"})

	// Fill labels with zero values
	for _, errorType := range errorTypes {
		metric.WithLabelValues(string(errorType))
	}

	return metric
}
