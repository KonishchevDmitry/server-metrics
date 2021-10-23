package cpu

import (
	"context"

	"github.com/pkg/math"
	"golang.org/x/xerrors"

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
	// We do this manual racy calculations as the best effort: on my server a get the following results:
	//
	// /sys/fs/cgroup# grep user_usec cpu.stat | cut -d ' ' -f 2
	// 46283360000
	// /sys/fs/cgroup# bc <<< $(grep user_usec */cpu.stat | cut -d ' ' -f 2 | tr '\n' '+' | sed 's/\+$//')
	// 51792306852
	//
	// As you can see, about 10% of root CPU usage is lost somewhere. So as a workaround we manually calculate diffs and
	// hope that they will be precise enough.

	current := rootUsage{root: root}

	{
		total := current.children.ToNamedUsage()

		for _, child := range children {
			childUsage, exists, err := c.collect(child)
			if err != nil {
				return err
			} else if !exists {
				return xerrors.Errorf("%q has been deleted during metrics collection", child.Path())
			}

			for index, usage := range childUsage.ToNamedUsage() {
				*total[index].Value += *usage.Value
			}
		}
	}

	if c.lastRootUsage != nil {
		diff := current

		{
			last := c.lastRootUsage.root.ToNamedUsage()

			for index, current := range diff.root.ToNamedUsage() {
				*current.Value -= *last[index].Value
				if *current.Value < 0 {
					return xerrors.Errorf("Got a negative %s", current.Name)
				}
			}
		}

		{
			last := c.lastRootUsage.children.ToNamedUsage()

			for index, current := range diff.children.ToNamedUsage() {
				*current.Value -= *last[index].Value
				if *current.Value < 0 {
					return xerrors.Errorf("Got a negative children %s", current.Name)
				}
			}
		}

		netRoot := diff.root
		netRootValues := netRoot.ToNamedUsage()

		for index, children := range diff.children.ToNamedUsage() {
			root := netRootValues[index]
			*root.Value = math.MaxInt64(0, *root.Value-*children.Value)
		}

		for index, netRootTotal := range c.netRootUsage.ToNamedUsage() {
			*netRootTotal.Value += *netRootValues[index].Value
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

var _ cgroups.ToNamedUsage = &Usage{}

func (u *Usage) ToNamedUsage() []cgroups.NamedUsage {
	return []cgroups.NamedUsage{
		cgroups.MakeNamedUsage("user CPU usage", &u.user, 0),
		cgroups.MakeNamedUsage("system CPU usage", &u.system, 0),
	}
}

type rootUsage struct {
	root     Usage
	children Usage
}
