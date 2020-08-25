package memory

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

const subsystem = "cpu"

var userMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "user",
		Help:      "CPU time consumed in user mode.",
	},
	[]string{metrics.ServiceLabel},
)

var systemMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "system",
		Help:      "CPU time consumed in system (kernel) mode.",
	},
	[]string{metrics.ServiceLabel},
)

func init() {
	prometheus.MustRegister(userMetric)
	prometheus.MustRegister(systemMetric)
}

type Collector struct {
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Controller() string {
	return "cpuacct"
}

func (c *Collector) Collect(ctx context.Context, slice *cgroups.Slice) (bool, error) {
	return true, nil
}
