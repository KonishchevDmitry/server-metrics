package collector

import (
	"context"
	"fmt"

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
		err = fmt.Errorf("%q is not mounted", root.Path())
	}
	if err != nil {
		logging.L(ctx).Errorf("Failed to observe cgroups hierarchy: %s.", err)
	}
}

func (c *Collector) observe(ctx context.Context, group *cgroups.Group, services map[string]string) (bool, error) {
	classification, classified, err := c.classifier.ClassifySlice(ctx, group.Name)
	if err != nil {
		logging.L(ctx).Errorf("Failed to classify %q cgroup: %s.", group.Name, err)
		return true, nil
	}
	needsCollection := classification.TotalCollection

	if classification.TotalCollection {
		// FIXME(konishchev): Exclude support
		if false {
			for _, name := range classification.TotalExcludeChildren {
				if _, err := c.observe(ctx, group.Child(name), services); err != nil {
					return false, err
				}
			}
		}
	} else {
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

	if otherGroup, ok := services[classification.Service]; ok {
		logging.L(ctx).Errorf("Both %q and %q resolve to %q service.", otherGroup, group.Name, classification.Service)
		return true, nil
	}
	services[classification.Service] = group.Name

	if exists, err := c.collect(ctx, classification.Service, group); err != nil {
		logging.L(ctx).Errorf("Failed to collect metrics for %s cgroup: %s.", group.Name, err)
	} else if !exists {
		return false, nil
	}

	return true, nil
}

// FIXME(konishchev): Exclude support
func (c *Collector) collect(ctx context.Context, service string, group *cgroups.Group) (bool, error) {
	logging.L(ctx).Debugf("Collecting %s as %s:", group.Name, service)

	for _, collector := range c.collectors {
		if exists, err := collector.Collect(ctx, service, group, nil); err != nil || !exists {
			return exists, err
		}
	}

	return true, nil
}
