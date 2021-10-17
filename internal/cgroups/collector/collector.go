package collector

import (
	"context"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cpu"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

type Collector struct {
	lastRootUsage    Usage
	rootUsageCounter uint64
}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Collect(ctx context.Context) {
	observe(ctx, c)
}

func (c *Collector) collect(ctx context.Context, service string, group *cgroups.Group) (bool, error) {
	logging.L(ctx).Debugf("Collecting %s -> %s:", group.Name, service)

	usage, exists, err := collect(group)
	if !exists || err != nil {
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

	cpu.Send(ctx, service, usage.cpu, stable)

	return true, nil
}

func (c *Collector) collectRoot(ctx context.Context, group *cgroups.Group, usage Usage) (Usage, bool, error) {
	children, exists, err := group.Children()
	if err != nil || !exists {
		return Usage{}, exists, err
	}

	netRootUsage := usage
	netRootNamedUsage := netRootUsage.ToNamedUsage()

	for _, child := range children {
		childUsage, exists, err := collect(child)
		if err != nil {
			return Usage{}, false, err
		} else if !exists {
			return Usage{}, false, xerrors.Errorf("%q has been deleted during metrics collection", child.Path())
		}

		childNamedUsage := childUsage.ToNamedUsage()

		for index, netRoot := range netRootNamedUsage {
			child := childNamedUsage[index]
			// FIXME(konishchev): Support
			if false && *netRoot.Value < *child.Value {
				return Usage{}, false, xerrors.Errorf("Got a negative %s", netRoot.Name)
			}
			*netRoot.Value -= *child.Value
		}
	}

	lastRootNamedUsage := c.lastRootUsage.ToNamedUsage()

	for index, current := range netRootNamedUsage {
		if !current.Monotonic {
			continue
		}

		last := lastRootNamedUsage[index]

		if diff := *current.Value - *last.Value; diff < 0 {
			calculationError := -diff

			if calculationError > current.AllowedError {
				logging.L(ctx).Warnf(
					"Calculated %s for root cgroup is less then previous: %d vs %d (%d).",
					current.Name, *current.Value, *last.Value, diff)
			}

			*current.Value = *last.Value
		}
	}

	c.lastRootUsage = netRootUsage
	c.rootUsageCounter++

	return netRootUsage, true, nil
}

type Usage struct {
	cpu cpu.Usage
}

var _ cgroups.ToNamedUsage = &Usage{}

func (u *Usage) ToNamedUsage() []cgroups.NamedUsage {
	return u.cpu.ToNamedUsage()
}

func collect(group *cgroups.Group) (Usage, bool, error) {
	var usage Usage
	var exists bool
	var err error

	usage.cpu, exists, err = cpu.Collect(group)
	if err != nil || !exists {
		return Usage{}, exists, err
	}

	return usage, true, nil
}
