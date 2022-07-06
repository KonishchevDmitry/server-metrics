package memory

import (
	"context"
	"fmt"

	"github.com/pkg/math"

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

func (c *Collector) Collect(ctx context.Context, service string, group *cgroups.Group, exclude []string) (bool, error) {
	usage, exists, err := c.collect(group)
	if err != nil || !exists {
		return exists, err
	}

	var isRoot, isOptionalChildren bool
	var children []*cgroups.Group

	if group.IsRoot() {
		isRoot = true
		children, exists, err = group.Children()
		if err != nil || !exists {
			return exists, err
		}
	} else if len(exclude) != 0 {
		isRoot, isOptionalChildren = true, true
		for _, name := range exclude {
			children = append(children, group.Child(name))
		}
	}

	if isRoot {
		usage, exists, err = c.collectRoot(usage, children, isOptionalChildren)
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

func (c *Collector) collectRoot(usage Usage, children []*cgroups.Group, isOptionalChildren bool) (Usage, bool, error) {
	rootUsages := usage.ToUsage()

	for _, child := range children {
		childUsage, exists, err := c.collect(child)
		if err != nil {
			return usage, false, err
		} else if !exists {
			if isOptionalChildren {
				continue
			}
			return usage, false, fmt.Errorf("%q has been deleted during metrics collection", child.Path())
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
