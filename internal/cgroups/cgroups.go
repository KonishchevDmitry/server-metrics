package cgroups

import (
	"context"
	"path"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Discover(ctx context.Context) {
	_, ok, err := walk(ctx, "/sys/fs/cgroup/cpu", "/", func(slice *slice) error {
		logging.L(ctx).Infof("%s", slice.name)
		return nil
	})
	if err != nil {
		panic(err) // FIXME
	} else if !ok {
		panic(ok) // FIXME
	}
}

func walk(ctx context.Context, root string, name string, handler func(slice *slice) error) (*slice, bool, error) {
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
		child, ok, err := walk(ctx, root, path.Join(name, childName), handler)
		if err != nil {
			return nil, false, err
		} else if ok {
			slice.children = append(slice.children, child)
		}
	}

	return slice, true, handler(slice)
}
