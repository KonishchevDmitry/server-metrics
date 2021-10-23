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

// FIXME(konishchev): Rename?
// Amount of memory used in anonymous mappings such as brk(), sbrk(), and mmap(MAP_ANONYMOUS)
// Amount of memory used to cache filesystem data, including tmpfs and shared memory.
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
	rootUsages := usage.ToNamedUsage()

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

		for index, childUsage := range childUsage.ToNamedUsage() {
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

var _ cgroups.ToNamedUsage = &Usage{}

func (u *Usage) ToNamedUsage() []cgroups.NamedUsage {
	return []cgroups.NamedUsage{
		cgroups.MakeNamedUsage("rss memory usage", &u.rss, 0),
		cgroups.MakeNamedUsage("page cache usage", &u.cache, 0),
		cgroups.MakeNamedUsage("kernel memory usage", &u.kernel, 0),
	}
}
