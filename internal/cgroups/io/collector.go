package io

import (
	"context"
	"fmt"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cgroupsutil"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
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

func (c *Collector) Reset() {
	c.resolver.reset()
}

func (c *Collector) Collect(ctx context.Context, service string, group *cgroups.Group, exclude []string) (bool, error) {
	var (
		isRoot             bool
		children           []*cgroups.Group
		isOptionalChildren bool
		exists             bool
		err                error
	)

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

	usage, exists, err := c.collect(group)
	if err != nil || !exists {
		return exists, err
	}

	if isRoot {
		usage, err = c.collectRoot(group, usage, children, isOptionalChildren)
		if err != nil {
			return true, err
		}
	}

	c.record(ctx, service, usage)
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

func (c *Collector) collectRoot(
	group *cgroups.Group, totalUsage Usage, children []*cgroups.Group, isOptionalChildren bool,
) (Usage, error) {
	current := make(map[string]*rootUsage, len(totalUsage))
	for device, usage := range totalUsage {
		current[device] = &rootUsage{root: *usage}
	}

	for _, child := range children {
		childUsage, exists, err := c.collect(child)
		if err != nil {
			return Usage{}, err
		} else if !exists {
			if isOptionalChildren {
				continue
			}
			return Usage{}, fmt.Errorf("%q has been deleted during metrics collection", child.Path())
		}

		for device, usage := range childUsage {
			total, ok := current[device]
			if !ok {
				total = &rootUsage{}
				current[device] = total
			}
			cgroups.AddUsage(&total.children, usage)
		}
	}

	state, ok := c.roots[group.Name]
	if ok && state.lastUsage != nil {
		for device, current := range current {
			last, ok := state.lastUsage[device]
			if !ok {
				last = &rootUsage{}
			}

			netRootUsage, ok := state.netUsage[device]
			if !ok {
				netRootUsage = &deviceUsage{}
				state.netUsage[device] = netRootUsage
			}

			if err := cgroups.CalculateRootGroupUsage(netRootUsage, current, last); err != nil {
				state.lastUsage = nil
				return Usage{}, err
			}
		}
	} else if ok {
		// We decided to forget last usage on previous call, so just obtain a new one
	} else {
		state = &rootState{netUsage: make(Usage)}
		c.roots[group.Name] = state
	}

	state.lastUsage = current
	return state.netUsage, nil
}

func (c *Collector) record(ctx context.Context, service string, usage Usage) {
	for device, stat := range usage {
		device = c.resolver.getDeviceName(ctx, device)

		logging.L(ctx).Debugf(
			"* %s: %s: reads=%d, writes=%d, read=%d, written=%d",
			service, device, stat.reads, stat.writes, stat.read, stat.written)

		labels := newLabels(service, device)

		readsMetric.With(labels).Set(float64(stat.read))
		writesMetric.With(labels).Set(float64(stat.writes))

		readBytesMetric.With(labels).Set(float64(stat.read))
		writtenBytesMetric.With(labels).Set(float64(stat.written))
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
	lastUsage map[string]*rootUsage
	netUsage  Usage
}
