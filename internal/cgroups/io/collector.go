package io

import (
	"context"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cgroupsutil"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

type Collector struct {
	resolver      *deviceResolver
	lastRootUsage map[string]*rootUsage
	netRootUsage  Usage
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{
		resolver:     newDeviceResolver(),
		netRootUsage: make(Usage),
	}
}

func (c *Collector) Reset() {
	c.resolver.reset()
}

func (c *Collector) Collect(ctx context.Context, service string, group *cgroups.Group) (bool, error) {
	var (
		children []*cgroups.Group
		exists   bool
		err      error
	)

	isRoot := group.IsRoot()

	if isRoot {
		children, exists, err = group.Children()
		if err != nil || !exists {
			return exists, err
		}
	}

	usage, exists, err := c.collect(group)
	if err != nil || !exists {
		return exists, err
	}

	if isRoot {
		if err := c.collectRoot(usage, children); err != nil {
			return true, err
		}
		usage = c.netRootUsage
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

func (c *Collector) collectRoot(root Usage, children []*cgroups.Group) error {
	current := make(map[string]*rootUsage, len(root))
	for device, usage := range root {
		current[device] = &rootUsage{root: *usage}
	}

	for _, child := range children {
		childUsage, exists, err := c.collect(child)
		if err != nil {
			return err
		} else if !exists {
			return xerrors.Errorf("%q has been deleted during metrics collection", child.Path())
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

	for device, current := range current {
		last, ok := c.lastRootUsage[device]
		if !ok {
			continue
		}

		netRootUsage, ok := c.netRootUsage[device]
		if !ok {
			netRootUsage = &deviceUsage{}
			c.netRootUsage[device] = netRootUsage
		}

		if err := cgroups.CalculateRootGroupUsage(netRootUsage, current, last); err != nil {
			return err
		}
	}

	c.lastRootUsage = current
	return nil
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
