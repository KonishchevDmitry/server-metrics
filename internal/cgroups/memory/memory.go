package memory

import (
	"context"
	"path"

	"github.com/c2h5oh/datasize"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

const controller = "memory"

var rssMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: controller,
		Name:      "rss",
		Help:      "Anonymous and swap cache memory usage.",
	},
	[]string{metrics.ServiceLabel},
)

var cacheMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
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

func (o *Collector) Controller() string {
	return controller
}

func (o *Collector) Collect(ctx context.Context, slice *cgroups.Slice) (bool, error) {
	statName := "memory.stat"
	stat, exists, err := cgroups.ReadStat(path.Join(slice.Path, statName))
	if !exists || err != nil {
		return exists, err
	}

	var getErr error
	get := func(name string) int64 {
		if slice.Total {
			name = "total_" + name
		}

		value, ok := stat[name]
		if !ok {
			getErr = xerrors.Errorf("%q entry of %q is missing", name, statName)
		}

		return value
	}

	cache := get("cache")
	rss := get("rss") + get("rss_huge")
	if getErr != nil {
		return false, getErr
	}

	logging.L(ctx).Debugf("* %s: rss=%s, cache=%s", slice.Service, datasize.ByteSize(rss), datasize.ByteSize(cache))
	rssMetric.With(metrics.Labels(slice.Service)).Set(float64(rss))
	cacheMetric.With(metrics.Labels(slice.Service)).Set(float64(cache))

	return true, nil
}
