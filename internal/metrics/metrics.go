package metrics

import "github.com/prometheus/client_golang/prometheus"

const Namespace = "server"

var ErrorsMetric = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: Namespace,
	Subsystem: "metrics",
	Name:      "errors",
	Help:      "Metrics collection errors.",
})

func init() {
	prometheus.MustRegister(ErrorsMetric)
}
