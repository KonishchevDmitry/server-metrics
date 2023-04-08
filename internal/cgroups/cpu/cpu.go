package cpu

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cgroupsutil"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

var userMetric = metrics.ServiceDesc("cpu", "user", "CPU time consumed in user mode.")
var systemMetric = metrics.ServiceDesc("cpu", "system", "CPU time consumed in system (kernel) mode.")

type Collector struct {
	roots map[string]*rootState
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{roots: make(map[string]*rootState)}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- userMetric
	descs <- systemMetric
}

func (c *Collector) Pre() {
	for _, state := range c.roots {
		state.collected = false
	}
}

func (c *Collector) Post(ctx context.Context) {
	for name, state := range c.roots {
		if !state.collected {
			if cgroups.NewGroup(name).IsRoot() {
				logging.L(ctx).Errorf("cpu: %q hasn't been collected.", name)
			} else {
				logging.L(ctx).Debugf("cpu: %q root hasn't been collected. Assuming it deleted and dropping its state.", name)
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

func (c *Collector) collectRoot(group *cgroups.Group, usage Usage, children []*cgroups.Group) (Usage, bool, error) {
	current := rootUsage{root: usage}

	for _, child := range children {
		child := child

		childUsage, childExists, err := c.collect(child)
		if err != nil {
			return Usage{}, false, err
		}

		if !childExists {
			if group.IsRoot() {
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

		cgroups.AddUsage(&current.children, &childUsage)
	}

	state, ok := c.roots[group.Name]
	if ok {
		if err := cgroups.CalculateRootGroupUsage(&state.netUsage, &current, &state.lastUsage); err != nil {
			return Usage{}, false, err
		}
	} else {
		state = &rootState{}
		c.roots[group.Name] = state
	}

	state.lastUsage = current
	state.collected = true

	return state.netUsage, true, nil
}

func (c *Collector) record(ctx context.Context, service string, usage Usage, metrics chan<- prometheus.Metric) {
	const usec = 1_000_000

	user := float64(usage.user) / usec
	system := float64(usage.system) / usec
	logging.L(ctx).Debugf("* %s: cpu: user=%.1fs, system=%.1fs", service, user, system)

	metrics <- prometheus.MustNewConstMetric(userMetric, prometheus.CounterValue, float64(user), service)
	metrics <- prometheus.MustNewConstMetric(systemMetric, prometheus.CounterValue, float64(system), service)
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
	collected bool
}
