// Since https://github.com/prometheus/node_exporter/pull/3049 Prometheus Node Exporter misses some /proc/meminfo
// metrics and doesn't hurry to fix that (https://github.com/prometheus/procfs/pull/655), so use our own exporter.

package meminfo

import (
	"context"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type Collector struct {
	logger *zap.SugaredLogger
}

var _ prometheus.Collector = &Collector{}

func NewCollector(logger *zap.SugaredLogger) *Collector {
	return &Collector{
		logger: logger,
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- meminfoMetric
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	ctx := logging.WithLogger(context.Background(), c.logger)

	if err := c.observe(ctx, metrics); err != nil {
		logging.L(ctx).Errorf("Failed to collect meminfo metrics: %s.", err)
	}
}

func (c *Collector) observe(ctx context.Context, metrics chan<- prometheus.Metric) error {
	meminfo, err := readMeminfo()
	if err != nil {
		return err
	}

	logging.L(ctx).Debugf("Meminfo:")
	for name, value := range meminfo {
		logging.L(ctx).Debugf("* %s: %d", name, value)
		metrics <- prometheus.MustNewConstMetric(meminfoMetric, prometheus.CounterValue, float64(value), name)
	}

	return nil
}
