package cgroups

import (
	"context"

	"golang.org/x/xerrors"
)

type observer interface {
	controller() string
	observe(ctx context.Context, slice *slice, serviceName string, total bool) (bool, error)
}

type baseObserver struct {
	metrics map[string]string
}

func makeBaseObserver() baseObserver {
	return baseObserver{make(map[string]string)}
}

func (o *baseObserver) observe(sliceName string, serviceName string) error {
	if otherName, ok := o.metrics[serviceName]; ok {
		return xerrors.Errorf("Both %q and %q resolve to %q service", otherName, sliceName, serviceName)
	}

	o.metrics[serviceName] = sliceName
	return nil
}
