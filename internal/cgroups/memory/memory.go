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

var rssMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: metrics.MemorySubsystem,
		Name:      "rss",
		Help:      "Anonymous and swap cache memory usage.",
	},
	[]string{metrics.ServiceLabel},
)

var cacheMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: metrics.MemorySubsystem,
		Name:      "cache",
		Help:      "Page cache memory usage.",
	},
	[]string{metrics.ServiceLabel},
)

func init() {
	prometheus.MustRegister(rssMetric)
	prometheus.MustRegister(cacheMetric)
}

type Observer struct {
	cgroups.BaseObserver
}

var _ cgroups.Observer = &Observer{}

func NewObserver() *Observer {
	return &Observer{cgroups.MakeBaseObserver()}
}

func (o *Observer) Controller() string {
	return "memory"
}

func (o *Observer) Observe(ctx context.Context, slice *cgroups.Slice, serviceName string, total bool) (bool, error) {
	if err := o.BaseObserver.Observe(slice.Name, serviceName); err != nil {
		logging.L(ctx).Errorf("%s.", err)
		return true, nil
	}

	if !total {
		if hasTasks, err := slice.HasTasks(ctx); err != nil {
			return false, err
		} else if !hasTasks {
			return true, nil
		}
	}

	statName := "memory.stat"
	stat, exists, err := cgroups.ReadStat(path.Join(slice.Path, statName))
	if !exists || err != nil {
		return exists, err
	}

	var getErr error
	get := func(name string) int64 {
		if total {
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

	logging.L(ctx).Debugf("* %s: rss=%s, cache=%s", serviceName, datasize.ByteSize(rss), datasize.ByteSize(cache))
	rssMetric.With(metrics.Labels(serviceName)).Set(float64(rss))
	cacheMetric.With(metrics.Labels(serviceName)).Set(float64(cache))

	return true, nil
}
