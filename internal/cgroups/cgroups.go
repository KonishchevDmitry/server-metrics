package cgroups

import (
	"context"
	"path"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Observe(ctx context.Context) {
	for _, observer := range []observer{newMemoryObserver()} {
		controller := observer.controller()
		rootPath := path.Join("/sys/fs/cgroup", controller)

		_, ok, err := walk(ctx, rootPath, "/", observer)
		if err == nil && !ok {
			err = xerrors.Errorf("%q is not mounted", rootPath)
		}
		if err != nil {
			logging.L(ctx).Errorf("Failed to observe %q cgroups controller: %s.", err)
		}
	}
}

type observer interface {
	controller() string
	observe(ctx context.Context, slice *slice) (bool, error)
}

func walk(ctx context.Context, root string, name string, observer observer) (*slice, bool, error) {
	slice := &slice{
		name: name,
		path: path.Join(root, name),
	}

	children, ok, err := listSlice(slice.path)
	if err != nil {
		return nil, false, err
	} else if !ok {
		logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.path)
		return nil, false, nil
	}

	for _, childName := range children {
		child, ok, err := walk(ctx, root, path.Join(name, childName), observer)
		if err != nil {
			return nil, false, err
		} else if ok {
			slice.children = append(slice.children, child)
		}
	}

	ok, err = observer.observe(ctx, slice)
	if err != nil {
		return nil, false, xerrors.Errorf("Failed to observe %q: %w", err)
	} else if !ok {
		logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.path)
		return nil, false, nil
	}

	return slice, true, nil
}
