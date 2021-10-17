package cpu

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var userMetric = metrics.NewServiceMetric("cpu", "user", "CPU time consumed in user mode.")
var systemMetric = metrics.NewServiceMetric("cpu", "system", "CPU time consumed in system (kernel) mode.")

func init() {
	prometheus.MustRegister(userMetric)
	prometheus.MustRegister(systemMetric)
}

type Usage struct {
	user   int64
	system int64
}

var _ cgroups.ToNamedUsage = &Usage{}

func (u *Usage) ToNamedUsage() []cgroups.NamedUsage {
	return []cgroups.NamedUsage{
		// FIXME(konishchev): Allowed error
		cgroups.MakeMonotonicNamedUsage("user CPU usage", &u.user, 0),
		cgroups.MakeMonotonicNamedUsage("system CPU usage", &u.system, 0),
	}
}

func Collect(group *cgroups.Group) (Usage, bool, error) {
	var usage Usage

	stats, exists, err := cgroups.ReadStat(group, "cpu.stat")
	if !exists || err != nil {
		return usage, exists, err
	}

	usage.user, err = stats.Get("user_usec")
	if err == nil {
		usage.system, err = stats.Get("system_usec")
	}

	return usage, true, err
}

func Send(ctx context.Context, service string, usage Usage, stable bool) {
	const usec = 1_000_000
	user := float64(usage.user) / usec
	system := float64(usage.system) / usec

	logging.L(ctx).Debugf("* %s: cpu: user=%.1fs, system=%.1fs", service, user, system)
	if stable {
		userMetric.With(metrics.Labels(service)).Set(user)
		systemMetric.With(metrics.Labels(service)).Set(system)
	}
}
