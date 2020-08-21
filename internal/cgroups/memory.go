package cgroups

import (
	"context"
	"path"

	"github.com/c2h5oh/datasize"
	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

type memoryObserver struct {
	baseObserver
}

var _ observer = &memoryObserver{}

func newMemoryObserver() *memoryObserver {
	return &memoryObserver{makeBaseObserver()}
}

func (o *memoryObserver) controller() string {
	return "memory"
}

func (o *memoryObserver) observe(ctx context.Context, slice *slice, metricName string, total bool) (bool, error) {
	if err := o.baseObserver.observe(slice.name, metricName); err != nil {
		logging.L(ctx).Errorf("%s.", err)
		return true, nil
	}

	if !total {
		if hasTasks, err := slice.hasTasks(ctx); err != nil {
			return false, err
		} else if !hasTasks {
			return true, nil
		}
	}

	statName := "memory.stat"
	stat, ok, err := readStat(path.Join(slice.path, statName))
	if !ok || err != nil {
		return ok, err
	}

	var getErr error
	get := func(name string) int64 {
		if total {
			name = "total_" + name
		}

		value, ok := stat[name]
		if !ok {
			getErr = xerrors.Errorf("%q entry of %q is missing", name, statName)
		}

		return value
	}

	cache := get("cache")
	rss := get("rss") + get("rss_huge")
	if getErr != nil {
		return false, getErr
	}

	logging.L(ctx).Infof("* %s: rss=%s, cache=%s", metricName, datasize.ByteSize(rss), datasize.ByteSize(cache))
	return true, nil
}
