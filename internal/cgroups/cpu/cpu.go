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
	roots map[string]*rootState
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{roots: make(map[string]*rootState)}
}

func (c *Collector) Reset() {
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

func (c *Collector) collectRoot(
	group *cgroups.Group, usage Usage, children []*cgroups.Group, isOptionalChildren bool,
) (Usage, error) {
	current := rootUsage{root: usage}

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
		cgroups.AddUsage(&current.children, &childUsage)
	}

	state, ok := c.roots[group.Name]
	if ok {
		if err := cgroups.CalculateRootGroupUsage(&state.netUsage, &current, &state.lastUsage); err != nil {
			return Usage{}, err
		}
	} else {
		state = &rootState{}
		c.roots[group.Name] = state
	}

	state.lastUsage = current
	return state.netUsage, nil
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

type rootState struct {
	lastUsage rootUsage
	netUsage  Usage
}
