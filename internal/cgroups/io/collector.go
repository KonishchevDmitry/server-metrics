package io

import (
	"context"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cgroupsutil"
	"github.com/KonishchevDmitry/server-metrics/internal/constants"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

type Collector struct {
	resolver         *deviceResolver
	lastRootUsage    Usage
	rootUsageCounter uint64
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{resolver: newDeviceResolver()}
}

func (c *Collector) Reset() {
	c.resolver.reset()
}

func (c *Collector) Collect(ctx context.Context, service string, group *cgroups.Group) (bool, error) {
	usage, exists, err := c.collect(group)
	if err != nil || !exists {
		return exists, err
	}

	stable := true
	if group.IsRoot() {
		usage, exists, err = c.collectRoot(ctx, group, usage)
		if err != nil || !exists {
			return exists, err
		}

		// Don't send first calculations to reduce chances to report value which is less than previous on daemon restart
		stable = c.rootUsageCounter >= 3
	}

	c.record(ctx, service, usage, stable)
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

func (c *Collector) collectRoot(ctx context.Context, group *cgroups.Group, usage Usage) (Usage, bool, error) {
	children, exists, err := group.Children()
	if err != nil || !exists {
		return nil, exists, err
	}

	for _, child := range children {
		childUsage, exists, err := c.collect(child)
		if err != nil {
			return nil, false, err
		} else if !exists {
			return nil, false, xerrors.Errorf("%q has been deleted during metrics collection", child.Path())
		}

		for device, childUsage := range childUsage {
			rootUsage, ok := usage[device]
			if !ok {
				// FIXME(konishchev): Support
				continue
				return nil, false, xerrors.Errorf(
					"Got %q device usage statistics for %q group which is missing on the root group",
					device, child.Name)
			}

			childNamedUsage := childUsage.ToNamedUsage()

			for index, rootUsage := range rootUsage.ToNamedUsage() {
				childUsage := childNamedUsage[index]
				if *rootUsage.Value < *childUsage.Value {
					return Usage{}, false, xerrors.Errorf("Got a negative %s for %s", rootUsage.Name, device)
				}
				*rootUsage.Value -= *childUsage.Value
			}
		}
	}

	for device, current := range usage {
		last, ok := c.lastRootUsage[device]
		if !ok {
			continue
		}

		lastNamed := last.ToNamedUsage()

		for index, current := range current.ToNamedUsage() {
			last := lastNamed[index]

			if diff := *current.Value - *last.Value; diff < 0 {
				calculationError := -diff

				if calculationError > current.Precision {
					logging.L(ctx).Warnf(
						"Calculated %s for root cgroup is less then previous: %d vs %d (%d).",
						current.Name, *current.Value, *last.Value, diff)
				}

				*current.Value = *last.Value
			}
		}
	}

	c.lastRootUsage = usage
	c.rootUsageCounter++

	return usage, true, nil
}

func (c *Collector) record(ctx context.Context, service string, usage Usage, stable bool) {
	for device, stat := range usage {
		device = c.resolver.getDeviceName(ctx, device)

		logging.L(ctx).Debugf(
			"* %s: %s: reads=%d, writes=%d, read=%d, written=%d",
			service, device, stat.reads, stat.writes, stat.read, stat.written)

		if stable {
			labels := newLabels(service, device)

			readsMetric.With(labels).Set(float64(stat.read))
			writesMetric.With(labels).Set(float64(stat.writes))

			readBytesMetric.With(labels).Set(float64(stat.read))
			writtenBytesMetric.With(labels).Set(float64(stat.written))
		}
	}
}

type Usage map[string]*deviceUsage

type deviceUsage struct {
	reads  int64
	writes int64

	read    int64
	written int64
}

var _ cgroups.ToNamedUsage = &deviceUsage{}

func (u *deviceUsage) ToNamedUsage() []cgroups.NamedUsage {
	return []cgroups.NamedUsage{
		// FIXME(konishchev): Alter errors

		cgroups.MakeNamedUsage("read operations", &u.reads, 200),
		cgroups.MakeNamedUsage("write operations", &u.writes, 200),

		cgroups.MakeNamedUsage("read bytes", &u.read, 10*constants.MB),
		cgroups.MakeNamedUsage("written bytes", &u.written, 10*constants.MB),
	}
}
