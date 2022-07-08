package cgroups

import "context"

type Collector interface {
	Reset()
	Collect(ctx context.Context, service string, group *Group, exclude []string) (bool, error)
	GC(ctx context.Context)
}
