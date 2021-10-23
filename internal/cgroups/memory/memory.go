package memory

import (
	"context"

	"github.com/pkg/math"
	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cgroupsutil"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var rssMetric = metrics.ServiceMetric("memory", "rss", "Anonymous and swap cache memory usage.")
var cacheMetric = metrics.ServiceMetric("memory", "cache", "Page cache memory usage.")
var kernelMetric = metrics.ServiceMetric("memory", "kernel", "Kernel data structures.")

type Collector struct {
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Reset() {
}

func (c *Collector) Collect(ctx context.Context, service string, group *cgroups.Group) (bool, error) {
	usage, exists, err := c.collect(group)
	if err != nil || !exists {
		return exists, err
	}

	if group.IsRoot() {
		usage, exists, err = c.collectRoot(group, usage)
		if err != nil || !exists {
			return exists, err
		}
	}

	c.record(ctx, service, usage)
	return true, nil
}

func (c *Collector) collect(group *cgroups.Group) (Usage, bool, error) {
	stat, exists, err := cgroupsutil.ReadStat(group, "memory.stat")
	if err != nil || !exists {
		return Usage{}, exists, err
	}

	var keyErr error
	get := func(name string) int64 {
		value, err := stat.Get(name)
		if err != nil {
			keyErr = err
		}
		return value
	}

	usage := Usage{
		rss:    get("anon"),
		cache:  get("file"),
		kernel: get("kernel_stack") + get("pagetables") + get("percpu") + get("slab_unreclaimable") + get("sock"),
	}
	if keyErr != nil {
		return Usage{}, true, keyErr
	}

	return usage, true, nil
}

func (c *Collector) collectRoot(group *cgroups.Group, usage Usage) (Usage, bool, error) {
	rootUsages := usage.ToUsage()

	children, exists, err := group.Children()
	if err != nil || !exists {
		return usage, exists, err
	}

	for _, child := range children {
		childUsage, exists, err := c.collect(child)
		if err != nil {
			return usage, false, err
		} else if !exists {
			return usage, false, xerrors.Errorf("%q has been deleted during metrics collection", child.Path())
		}

		for index, childUsage := range childUsage.ToUsage() {
			rootUsage := rootUsages[index]
			*rootUsage.Value = math.MaxInt64(0, *rootUsage.Value-*childUsage.Value)
		}
	}

	return usage, true, nil
}

func (c *Collector) record(ctx context.Context, service string, usage Usage) {
	logging.L(ctx).Debugf("* %s: memory: rss=%d, cache=%d, kernel=%d", service, usage.rss, usage.cache, usage.kernel)

	labels := metrics.ServiceLabels(service)
	rssMetric.With(labels).Set(float64(usage.rss))
	cacheMetric.With(labels).Set(float64(usage.cache))
	kernelMetric.With(labels).Set(float64(usage.kernel))
}

type Usage struct {
	rss    int64
	cache  int64
	kernel int64
}

var _ cgroups.ToUsage = &Usage{}

func (u *Usage) ToUsage() []cgroups.Usage {
	return []cgroups.Usage{
		cgroups.MakeUsage("rss memory usage", &u.rss),
		cgroups.MakeUsage("page cache usage", &u.cache),
		cgroups.MakeUsage("kernel memory usage", &u.kernel),
	}
}
