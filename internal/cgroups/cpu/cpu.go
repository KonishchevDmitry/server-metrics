package cpu

import (
	"context"
	"fmt"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cgroupsutil"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var userMetric = metrics.ServiceMetric("cpu", "user", "CPU time consumed in user mode.")
var systemMetric = metrics.ServiceMetric("cpu", "system", "CPU time consumed in system (kernel) mode.")

type Collector struct {
	lastRootUsage *rootUsage
	netRootUsage  Usage
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Reset() {
}

// FIXME(konishchev): Exclude support
func (c *Collector) Collect(ctx context.Context, service string, group *cgroups.Group, exclude []string) (bool, error) {
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
	stats, exists, err := cgroupsutil.ReadStat(group, "cpu.stat")
	if err != nil || !exists {
		return Usage{}, exists, err
	}

	var usage Usage
	usage.user, err = stats.Get("user_usec")
	if err == nil {
		usage.system, err = stats.Get("system_usec")
	}
	if err != nil {
		return Usage{}, true, err
	}

	return usage, true, nil
}

func (c *Collector) collectRoot(root Usage, children []*cgroups.Group) error {
	current := rootUsage{root: root}

	for _, child := range children {
		childUsage, exists, err := c.collect(child)
		if err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("%q has been deleted during metrics collection", child.Path())
		}
		cgroups.AddUsage(&current.children, &childUsage)
	}

	if c.lastRootUsage != nil {
		if err := cgroups.CalculateRootGroupUsage(&c.netRootUsage, &current, c.lastRootUsage); err != nil {
			return err
		}
	}

	c.lastRootUsage = &current
	return nil
}

func (c *Collector) record(ctx context.Context, service string, usage Usage) {
	const usec = 1_000_000

	user := float64(usage.user) / usec
	system := float64(usage.system) / usec
	logging.L(ctx).Debugf("* %s: cpu: user=%.1fs, system=%.1fs", service, user, system)

	labels := metrics.ServiceLabels(service)
	userMetric.With(labels).Set(user)
	systemMetric.With(labels).Set(system)
}

type Usage struct {
	user   int64
	system int64
}

var _ cgroups.ToUsage = &Usage{}

func (u *Usage) ToUsage() []cgroups.Usage {
	return []cgroups.Usage{
		cgroups.MakeUsage("user CPU usage", &u.user),
		cgroups.MakeUsage("system CPU usage", &u.system),
	}
}

type rootUsage struct {
	root     Usage
	children Usage
}

var _ cgroups.ToRootUsage = &rootUsage{}

func (u *rootUsage) ToRootUsage() (cgroups.ToUsage, cgroups.ToUsage) {
	return &u.root, &u.children
}
