package cgroups

import "context"

type Collector interface {
	// FIXME(konishchev): Reset missing roots
	Reset()
	Collect(ctx context.Context, service string, group *Group, exclude []string) (bool, error)
}
