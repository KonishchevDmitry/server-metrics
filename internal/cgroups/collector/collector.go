package collector

import (
	"context"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/classifier"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cpu"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/io"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/memory"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

type Collector struct {
	classifier *classifier.Classifier
	collectors []cgroups.Collector
}

func NewCollector(classifier *classifier.Classifier) *Collector {
	return &Collector{
		classifier: classifier,
		collectors: []cgroups.Collector{
			cpu.NewCollector(),
			memory.NewCollector(),
			io.NewCollector(),
		},
	}
}

func (c *Collector) Collect(ctx context.Context) {
	for _, collector := range c.collectors {
		collector.Reset()
	}

	root := cgroups.NewGroup("/")
	services := make(map[string]string)

	exists, err := c.observe(ctx, root, services)
	if err == nil && !exists {
		err = xerrors.Errorf("%q is not mounted", root.Path())
	}
	if err != nil {
		logging.L(ctx).Errorf("Failed to observe cgroups hierarchy: %s.", err)
	}
}

func (c *Collector) observe(ctx context.Context, group *cgroups.Group, services map[string]string) (bool, error) {
	service, totalCollection, classified, err := c.classifier.ClassifySlice(ctx, group.Name)
	if err != nil {
		logging.L(ctx).Errorf("Failed to classify %q cgroup: %s.", group.Name, err)
		return true, nil
	}
	needsCollection := totalCollection

	if !totalCollection {
		var observeChildren bool

		if group.IsRoot() {
			needsCollection = true
			observeChildren = true
		} else {
			hasProcesses, exists, err := group.HasProcesses()
			if err != nil || !exists {
				return exists, err
			}

			needsCollection = hasProcesses
			observeChildren = !hasProcesses
		}

		if observeChildren {
			children, exists, err := group.Children()
			if err != nil || !exists {
				return exists, err
			}

			for _, child := range children {
				if exists, err := c.observe(ctx, child, services); err != nil {
					return false, err
				} else if !exists {
					logging.L(ctx).Debugf("%q has been deleted during discovering.", child.Path())
				}
			}
		}
	}

	if !needsCollection {
		return true, nil
	} else if !classified {
		logging.L(ctx).Errorf("Unable to classify %q cgroup.", group.Name)
		return true, nil
	}

	if otherGroup, ok := services[service]; ok {
		logging.L(ctx).Errorf("Both %q and %q resolve to %q service.", otherGroup, group.Name, service)
		return true, nil
	}
	services[service] = group.Name

	if exists, err := c.collect(ctx, service, group); err != nil {
		logging.L(ctx).Errorf("Failed to collect metrics for %s cgroup: %s.", group.Name, err)
	} else if !exists {
		return false, nil
	}

	return true, nil
}

func (c *Collector) collect(ctx context.Context, service string, group *cgroups.Group) (bool, error) {
	logging.L(ctx).Debugf("Collecting %s as %s:", group.Name, service)

	for _, collector := range c.collectors {
		if exists, err := collector.Collect(ctx, service, group); err != nil || !exists {
			return exists, err
		}
	}

	return true, nil
}
