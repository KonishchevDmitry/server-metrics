package memory

import (
	"context"

	"github.com/c2h5oh/datasize"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

const controller = "memory"

var rssMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.ServicesNamespace,
		Subsystem: controller,
		Name:      "rss",
		Help:      "Anonymous and swap cache memory usage.",
	},
	[]string{metrics.ServiceLabel},
)

var cacheMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.ServicesNamespace,
		Subsystem: controller,
		Name:      "cache",
		Help:      "Page cache memory usage.",
	},
	[]string{metrics.ServiceLabel},
)

func init() {
	prometheus.MustRegister(rssMetric)
	prometheus.MustRegister(cacheMetric)
}

type Collector struct {
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Controller() string {
	return controller
}

func (c *Collector) Collect(ctx context.Context, slice *cgroups.Slice) (bool, error) {
	stat, exists, err := cgroups.ReadStat(slice, "memory.stat")
	if !exists || err != nil {
		return exists, err
	}

	var prefix string
	if slice.Total {
		prefix = "total_"
	}

	cache, err := stat.Get(prefix + "cache")
	if err != nil {
		return false, err
	}

	rssOrdinary, err := stat.Get(prefix + "rss")
	if err != nil {
		return false, err
	}

	rssHuge, err := stat.Get(prefix + "rss_huge")
	if err != nil {
		return false, err
	}

	rss := rssOrdinary + rssHuge
	logging.L(ctx).Debugf("* %s: rss=%s, cache=%s", slice.Service, datasize.ByteSize(rss), datasize.ByteSize(cache))

	rssMetric.With(metrics.Labels(slice.Service)).Set(float64(rss))
	cacheMetric.With(metrics.Labels(slice.Service)).Set(float64(cache))

	return true, nil
}
