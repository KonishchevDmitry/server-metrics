package collector

import (
	"context"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Collect(ctx context.Context) {
	// FIXME(konishchev): Enable
	//blkio.NewCollector(),
	//cpu.NewCollector(),
	//memory.NewCollector(),

	root := cgroups.NewGroup("/sys/fs/cgroup", "/")

	observer := observer{
		services:   make(map[string]string),
		collectors: []cgroups.Collector{},
	}

	exists, err := observer.observe(ctx, root)
	if err == nil && !exists {
		err = xerrors.Errorf("%q is not mounted", root.Path())
	}
	if err != nil {
		logging.L(ctx).Errorf("Failed to observe cgroups hierarchy: %s.", err)
	}
}

type observer struct {
	services   map[string]string
	collectors []cgroups.Collector
}

func (o *observer) observe(ctx context.Context, group *cgroups.Group) (bool, error) {
	service, totalCollection := cgroups.ClassifySlice(group.Name)
	needsCollection := totalCollection

	if !totalCollection {
		var observeChildren bool

		if group.IsRoot() {
			needsCollection = true
			observeChildren = true
		} else {
			hasProcesses, exists, err := group.HasProcesses()
			if err != nil {
				return false, err
			} else if !exists {
				return false, nil
			}

			needsCollection = hasProcesses
			observeChildren = !hasProcesses
		}

		if observeChildren {
			children, exists, err := group.Children()
			if err != nil {
				return false, err
			} else if !exists {
				return false, nil
			}

			for _, name := range children {
				child := group.Child(name)

				if exists, err := o.observe(ctx, child); err != nil {
					return false, err
				} else if !exists {
					logging.L(ctx).Debugf("%q has been deleted during discovering.", child.Path())
				}
			}
		}
	}

	if otherGroup, ok := o.services[service]; ok {
		logging.L(ctx).Errorf("Both %q and %q resolve to %q service.", otherGroup, group.Name, service)
		return true, nil
	}
	o.services[group.Service] = group.Name

	if needsCollection {
		logging.L(ctx).Debugf("Collecting %s -> %s:", group.Name, service)

		for _, collector := range o.collectors {
			if exists, err := collector.Collect(ctx, service, group); err != nil {
				logging.L(ctx).Errorf(
					"Failed to observe %s controller of %q: %s.",
					collector.Controller(), group.Path(), err)
			} else if !exists {
				return false, nil
			}
		}
	}

	return true, nil
}
