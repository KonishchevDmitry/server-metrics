package cgroups

import (
	"context"
	"path"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Collect(ctx context.Context) {
	for _, observer := range []observer{newMemoryObserver()} {
		controller := observer.controller()
		rootPath := path.Join("/sys/fs/cgroup", controller)

		logging.L(ctx).Debugf("%s controller:", controller)
		_, exists, err := walk(ctx, rootPath, "/", observer)
		if err == nil && !exists {
			err = xerrors.Errorf("%q is not mounted", rootPath)
		}
		if err != nil {
			logging.L(ctx).Errorf("Failed to observe %q cgroups controller: %s.", err)
		}
	}
}

func walk(ctx context.Context, root string, name string, observer observer) (*slice, bool, error) {
	serviceName, total := classifySlice(name)

	slice := &slice{
		name: name,
		path: path.Join(root, name),
	}

	if !total {
		children, exists, err := listSlice(slice.path)
		if err != nil {
			return nil, false, err
		} else if !exists {
			logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.path)
			return nil, false, nil
		}

		for _, childName := range children {
			child, exists, err := walk(ctx, root, path.Join(name, childName), observer)
			if err != nil {
				return nil, false, err
			} else if exists {
				slice.children = append(slice.children, child)
			}
		}
	}

	if exists, err := observer.observe(ctx, slice, serviceName, total); err != nil {
		return nil, false, xerrors.Errorf("Failed to observe %q: %w", slice.path, err)
	} else if !exists {
		logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.path)
		return nil, false, nil
	}

	return slice, true, nil
}

func classifySlice(name string) (string, bool) {
	var serviceName string
	var total bool

	switch name {
	case "/":
		serviceName = "kernel"
	case "/docker":
		serviceName = "docker-containers"
		total = true
	case "/init.scope":
		serviceName = "init"
	case "/user.slice":
		serviceName = "user"
		total = true
	default:
		if strings.HasPrefix(name, "/system.slice/") {
			serviceName = path.Base(name)
			serviceName = strings.TrimSuffix(serviceName, ".service")
			serviceName = strings.ReplaceAll(serviceName, `\x2d`, `-`)
		} else {
			serviceName = name
		}
	}

	return serviceName, total
}
