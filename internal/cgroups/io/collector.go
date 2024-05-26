package io

import (
	"context"
	"fmt"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cgroupsutil"
	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

type Collector struct {
	resolver *deviceResolver
	roots    map[string]*rootState
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{
		resolver: newDeviceResolver(),
		roots:    make(map[string]*rootState),
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- readsMetric
	descs <- writesMetric

	descs <- readBytesMetric
	descs <- writtenBytesMetric
}

func (c *Collector) Pre() {
	c.resolver.reset()
	for _, state := range c.roots {
		state.collected = false
	}
}

func (c *Collector) Post(ctx context.Context) {
	for name, state := range c.roots {
		if !state.collected {
			if cgroups.NewGroup(name).IsRoot() {
				logging.L(ctx).Errorf("io: %q hasn't been collected.", name)
			} else {
				logging.L(ctx).Debugf("io: %q root hasn't been collected. Assuming it deleted and dropping its state.", name)
				delete(c.roots, name)
			}
		}
	}
}

func (c *Collector) Collect(
	ctx context.Context, service string, group *cgroups.Group, exclude []string, metrics chan<- prometheus.Metric,
) (bool, error) {
	var (
		isRoot   bool
		children []*cgroups.Group
		exists   bool
		err      error
	)

	if group.IsRoot() {
		isRoot = true
		children, exists, err = group.Children()
		if err != nil || !exists {
			return exists, err
		}
	} else if len(exclude) != 0 {
		isRoot = true
		for _, name := range exclude {
			children = append(children, group.Child(name))
		}
	}

	usage, exists, err := c.collect(group)
	if err != nil || !exists {
		return exists, err
	}

	if isRoot {
		usage, exists, err = c.collectRoot(group, usage, children)
		if err != nil || !exists {
			return exists, err
		}
	}

	c.record(ctx, service, usage, metrics)
	return true, nil
}

func (c *Collector) collect(group *cgroups.Group) (Usage, bool, error) {
	stats, exists, err := cgroupsutil.ReadNamedStat(group, "io.stat")
	if err != nil || !exists {
		return Usage{}, exists, err
	}

	usage := make(Usage, len(stats))

	for device, stat := range stats {
		stat := stat

		var keyErr error
		get := func(name string) int64 {
			value, err := stat.Get(name)
			if err != nil {
				keyErr = err
			}
			return value
		}

		usage[device] = &deviceUsage{
			reads:  get("rios"),
			writes: get("wios"),

			read:    get("rbytes"),
			written: get("wbytes"),
		}
		if keyErr != nil {
			return Usage{}, true, keyErr
		}
	}

	return usage, true, nil
}

func (c *Collector) collectRoot(group *cgroups.Group, totalUsage Usage, children []*cgroups.Group) (Usage, bool, error) {
	isRoot := group.IsRoot()

	currentUsage := make(map[string]*rootUsage, len(totalUsage))
	for device, usage := range totalUsage {
		currentUsage[device] = &rootUsage{root: *usage}
	}

	for _, child := range children {
		child := child

		childUsage, childExists, err := c.collect(child)
		if err != nil {
			return Usage{}, false, err
		}

		if !childExists {
			if isRoot {
				return Usage{}, false, fmt.Errorf("%q has been deleted during metrics collection", child.Path())
			}

			// It might be a race with user session opening/closing
			if err := util.RetryRace(fmt.Errorf("%q is missing, but is expected to exist", child.Path()), func() (bool, error) {
				if rootExists, err := group.IsExist(); err != nil || !rootExists {
					return !rootExists, err
				}

				// Attention: Override the collection result
				childUsage, childExists, err = c.collect(child)
				return childExists, err
			}); err != nil || !childExists {
				return Usage{}, false, err
			}
		}

		for device, usage := range childUsage {
			total, ok := currentUsage[device]
			if !ok {
				total = &rootUsage{}
				currentUsage[device] = total
			}
			cgroups.AddUsage(&total.children, usage)
		}

	}

	var currentChildren map[string]struct{}
	if isRoot {
		currentChildren = make(map[string]struct{}, len(children))
		for _, child := range children {
			currentChildren[child.Name] = struct{}{}
		}
	}

	state, ok := c.roots[group.Name]
	if !ok {
		c.roots[group.Name] = &rootState{
			lastChildren: currentChildren,
			lastUsage:    currentUsage,
			netUsage:     make(Usage),
			collected:    true,
		}
		return nil, true, nil
	}

	lastChildren, lastUsage := state.lastChildren, state.lastUsage
	state.lastChildren = currentChildren
	state.lastUsage = currentUsage
	state.collected = true

	if isRoot {
		for name := range lastChildren {
			if _, ok := currentChildren[name]; !ok {
				return Usage{}, false, fmt.Errorf("%q has been deleted between metrics collection", name)
			}
		}
	}

	for device, current := range currentUsage {
		last, ok := lastUsage[device]
		if !ok {
			last = &rootUsage{}
		}

		netRootUsage, ok := state.netUsage[device]
		if !ok {
			netRootUsage = &deviceUsage{}
			state.netUsage[device] = netRootUsage
		}

		if err := cgroups.CalculateRootGroupUsage(netRootUsage, current, last); err != nil {
			return Usage{}, false, fmt.Errorf("%s device: %w", device, err)
		}
	}

	return state.netUsage, true, nil
}

func (c *Collector) record(ctx context.Context, service string, usage Usage, metrics chan<- prometheus.Metric) {
	for device, stat := range usage {
		device = c.resolver.getDeviceName(ctx, device)

		logging.L(ctx).Debugf(
			"* %s: %s: reads=%d, writes=%d, read=%d, written=%d",
			service, device, stat.reads, stat.writes, stat.read, stat.written)

		metrics <- prometheus.MustNewConstMetric(readsMetric, prometheus.CounterValue, float64(stat.reads), service, device)
		metrics <- prometheus.MustNewConstMetric(writesMetric, prometheus.CounterValue, float64(stat.writes), service, device)

		metrics <- prometheus.MustNewConstMetric(readBytesMetric, prometheus.CounterValue, float64(stat.read), service, device)
		metrics <- prometheus.MustNewConstMetric(writtenBytesMetric, prometheus.CounterValue, float64(stat.written), service, device)
	}
}

type Usage map[string]*deviceUsage

type deviceUsage struct {
	reads  int64
	writes int64

	read    int64
	written int64
}

var _ cgroups.ToUsage = &deviceUsage{}

func (u *deviceUsage) ToUsage() []cgroups.Usage {
	return []cgroups.Usage{
		cgroups.MakeUsage("read operations", &u.reads),
		cgroups.MakeUsage("write operations", &u.writes),

		cgroups.MakeUsage("read bytes", &u.read),
		cgroups.MakeUsage("written bytes", &u.written),
	}
}

type rootUsage struct {
	root     deviceUsage
	children deviceUsage
}

var _ cgroups.ToRootUsage = &rootUsage{}

func (u *rootUsage) ToRootUsage() (cgroups.ToUsage, cgroups.ToUsage) {
	return &u.root, &u.children
}

type rootState struct {
	lastChildren map[string]struct{}
	lastUsage    map[string]*rootUsage
	netUsage     Usage
	collected    bool
}
