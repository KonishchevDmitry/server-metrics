package cgroups

import (
	"context"

	"golang.org/x/xerrors"
)

type observer interface {
	controller() string
	observe(ctx context.Context, slice *slice, metricName string, total bool) (bool, error)
}

type baseObserver struct {
	metrics map[string]string
}

func makeBaseObserver() baseObserver {
	return baseObserver{make(map[string]string)}
}

func (o *baseObserver) observe(sliceName string, metricName string) error {
	if otherName, ok := o.metrics[metricName]; ok {
		return xerrors.Errorf("Both %q and %q results to %q metric", otherName, sliceName, metricName)
	}

	o.metrics[metricName] = sliceName
	return nil
}
