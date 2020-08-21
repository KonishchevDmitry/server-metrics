package cgroups

import (
	"context"
	"path"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

func Observe(ctx context.Context) {
	for _, observer := range []observer{newMemoryObserver()} {
		controller := observer.controller()
		rootPath := path.Join("/sys/fs/cgroup", controller)

		logging.L(ctx).Debugf("%s controller:", controller)
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
	observe(ctx context.Context, slice *slice, metricName string, total bool) (bool, error)
}

func walk(ctx context.Context, root string, name string, observer observer) (*slice, bool, error) {
	metricName, total := classifySlice(name)

	slice := &slice{
		name: name,
		path: path.Join(root, name),
	}

	if !total {
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
	}

	if ok, err := observer.observe(ctx, slice, metricName, total); err != nil {
		return nil, false, xerrors.Errorf("Failed to observe %q: %w", err)
	} else if !ok {
		logging.L(ctx).Debugf("%q has been deleted during discovering.", slice.path)
		return nil, false, nil
	}

	return slice, true, nil
}

// FIXME: Check for duplicates
func classifySlice(name string) (string, bool) {
	var metricName string
	var total bool

	switch name {
	case "/":
		metricName = "kernel"
	case "/docker":
		metricName = "docker"
		total = true
	case "/init.scope":
		metricName = "init"
	case "/user.slice":
		metricName = "user"
		total = true
	default:
		if strings.HasPrefix(name, "/system.slice/") {
			metricName = path.Base(name)
			metricName = strings.TrimSuffix(metricName, ".service")
			metricName = strings.ReplaceAll(metricName, `\x2d`, `-`)
		} else {
			metricName = name
		}
	}

	return metricName, total
}
