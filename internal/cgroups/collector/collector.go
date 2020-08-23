package collector

import (
	"context"
	"path"
	"sync"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/blkio"

	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/memory"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Collect(ctx context.Context, serial bool) {
	var lock sync.Mutex
	var group errgroup.Group
	defer func() {
		_ = group.Wait()
	}()

	for _, collector := range []cgroups.Collector{
		blkio.NewCollector(),
		memory.NewCollector(),
	} {
		observer := newObserver(collector)
		group.Go(func() error {
			if serial {
				lock.Lock()
				defer lock.Unlock()
				logging.L(ctx).Debugf("%s controller:", observer.collector.Controller())
			}
			observer.observe(ctx)
			return nil
		})
	}
}

type observer struct {
	rootPath  string
	services  map[string]string
	collector cgroups.Collector
}

func newObserver(collector cgroups.Collector) *observer {
	return &observer{
		rootPath:  path.Join("/sys/fs/cgroup", collector.Controller()),
		services:  make(map[string]string),
		collector: collector,
	}
}

func (o *observer) observe(ctx context.Context) {
	_, exists, err := o.observeSlice(ctx, "/")
	if err == nil && !exists {
		err = xerrors.Errorf("%q is not mounted", o.rootPath)
	}
	if err != nil {
		logging.L(ctx).Errorf("Failed to observe %q cgroups controller: %s.", o.collector.Controller(), err)
	}
}

func (o *observer) observeSlice(ctx context.Context, name string) (*cgroups.Slice, bool, error) {
	slice := cgroups.NewSlice(o.rootPath, name)

	if !slice.Total {
		children, exists, err := cgroups.ListSlice(slice.Path)
		if err != nil {
			return nil, false, err
		} else if !exists {
			logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.Path)
			return nil, false, nil
		}

		for _, childName := range children {
			child, exists, err := o.observeSlice(ctx, path.Join(name, childName))
			if err != nil {
				return nil, false, err
			} else if exists {
				slice.Children = append(slice.Children, child)
			}
		}
	}

	if collect, err := o.needsCollection(ctx, slice); err != nil {
		return nil, false, err
	} else if collect {
		if exists, err := o.collector.Collect(ctx, slice); err != nil {
			return nil, false, xerrors.Errorf("Failed to observe %q: %w", slice.Path, err)
		} else if !exists {
			logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.Path)
			return nil, false, nil
		}
	}

	return slice, true, nil
}

func (o *observer) needsCollection(ctx context.Context, slice *cgroups.Slice) (bool, error) {
	if otherName, ok := o.services[slice.Service]; ok {
		logging.L(ctx).Errorf("Both %q and %q resolve to %q service", otherName, slice.Name, slice.Service)
		return false, nil
	}
	o.services[slice.Service] = slice.Name

	if !slice.Total {
		if hasTasks, err := slice.HasTasks(ctx); err != nil {
			return false, err
		} else if !hasTasks {
			return false, nil
		}
	}

	return true, nil
}
