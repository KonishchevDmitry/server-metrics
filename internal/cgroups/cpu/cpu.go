package cpu

import (
	"context"
	"sync"

	"github.com/tklauser/go-sysconf"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
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
	usage, exists, err := readStat(slice)
	if !exists || err != nil {
		return exists, err
	}

	if slice.Name == "/" {
		var err error

		usage, err = collectRoot(slice, usage)
		if err != nil {
			return false, xerrors.Errorf("Failed to collect root cgroup CPU usage: %w", err)
		}
	} else if !slice.Total && len(slice.Children) != 0 {
		logging.L(ctx).Warnf("Calculating total CPU usage for %q which has child groups.", slice.Name)
	}

	hz, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil {
		return true, xerrors.Errorf("Unable to determine SC_CLK_TCK value")
	}

	user := float64(usage.user) / float64(hz)
	system := float64(usage.system) / float64(hz)
	logging.L(ctx).Debugf("* %s: user=%.0fs, system=%.0fs", slice.Service, user, system)

	userMetric.With(metrics.Labels(slice.Service)).Set(user)
	systemMetric.With(metrics.Labels(slice.Service)).Set(system)

	return true, nil
}

var lastRootUsage stat
var lastRootUsageLock sync.Mutex

func collectRoot(slice *cgroups.Slice, usage stat) (stat, error) {
	for _, child := range slice.Children {
		childUsage, exists, err := readStat(slice)
		if err != nil {
			return usage, err
		} else if !exists {
			return usage, xerrors.Errorf("%q has been deleted during metrics collection", child.Path)
		}

		for _, usages := range []struct {
			total *int64
			child *int64
		}{
			{&usage.user, &childUsage.user},
			{&usage.system, &childUsage.system},
		} {
			total, child := usages.total, usages.child
			if *total < *child {
				return usage, xerrors.Errorf("Got a negative CPU usage")
			}
			*total -= *child
		}
	}

	lastRootUsageLock.Lock()
	defer lastRootUsageLock.Unlock()

	for _, usages := range []struct {
		last    *int64
		current *int64
	}{
		{&lastRootUsage.user, &usage.user},
		{&lastRootUsage.system, &usage.system},
	} {
		if *usages.current < *usages.last {
			return usage, xerrors.Errorf("Got CPU usage less then previous")
		}
	}

	lastRootUsage = usage

	return usage, nil
}

type stat struct {
	user   int64
	system int64
}

func readStat(slice *cgroups.Slice) (stat, bool, error) {
	var usage stat

	stats, exists, err := cgroups.ReadStat(slice, "cpuacct.stat")
	if !exists || err != nil {
		return usage, exists, err
	}

	usage.user, err = stats.Get("user")
	if err == nil {
		usage.system, err = stats.Get("system")
	}

	return usage, true, err
}
