package collector

import (
	"context"
	"path"
	"strings"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/memory"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Collect(ctx context.Context) {
	for _, observer := range []cgroups.Observer{memory.NewObserver()} {
		controller := observer.Controller()
		rootPath := path.Join("/sys/fs/cgroup", controller)

		logging.L(ctx).Debugf("%s controller:", controller)
		_, exists, err := observe(ctx, rootPath, "/", observer)
		if err == nil && !exists {
			err = xerrors.Errorf("%q is not mounted", rootPath)
		}
		if err != nil {
			logging.L(ctx).Errorf("Failed to observe %q cgroups controller: %s.", err)
		}
	}
}

func observe(ctx context.Context, root string, name string, observer cgroups.Observer) (*cgroups.Slice, bool, error) {
	serviceName, total := classifySlice(name)

	slice := &cgroups.Slice{
		Name: name,
		Path: path.Join(root, name),
	}

	if !total {
		children, exists, err := cgroups.ListSlice(slice.Path)
		if err != nil {
			return nil, false, err
		} else if !exists {
			logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.Path)
			return nil, false, nil
		}

		for _, childName := range children {
			child, exists, err := observe(ctx, root, path.Join(name, childName), observer)
			if err != nil {
				return nil, false, err
			} else if exists {
				slice.Children = append(slice.Children, child)
			}
		}
	}

	if exists, err := observer.Observe(ctx, slice, serviceName, total); err != nil {
		return nil, false, xerrors.Errorf("Failed to observe %q: %w", slice.Path, err)
	} else if !exists {
		logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.Path)
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
