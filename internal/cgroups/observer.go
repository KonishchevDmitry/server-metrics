package cgroups

import (
	"context"

	"golang.org/x/xerrors"
)

type Observer interface {
	Controller() string
	Observe(ctx context.Context, slice *Slice, serviceName string, total bool) (bool, error)
}

type BaseObserver struct {
	metrics map[string]string
}

func MakeBaseObserver() BaseObserver {
	return BaseObserver{make(map[string]string)}
}

func (o *BaseObserver) Observe(sliceName string, serviceName string) error {
	if otherName, ok := o.metrics[serviceName]; ok {
		return xerrors.Errorf("Both %q and %q resolve to %q service", otherName, sliceName, serviceName)
	}

	o.metrics[serviceName] = sliceName
	return nil
}
