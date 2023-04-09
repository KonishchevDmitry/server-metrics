package cpu

import "github.com/prometheus/client_golang/prometheus"

var userMetric = prometheus.NewDesc(
	"cpu_user", "CPU time consumed in user mode.",
	metricLabels, nil)

var systemMetric = prometheus.NewDesc(
	"cpu_system", "CPU time consumed in system (kernel) mode.",
	metricLabels, nil)

var metricLabels = []string{"service"}
