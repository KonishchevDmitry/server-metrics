package cgroups

import (
	"context"
)

type Collector interface {
	Controller() string
	Collect(ctx context.Context, service string, slice *Group) (bool, error)
}
