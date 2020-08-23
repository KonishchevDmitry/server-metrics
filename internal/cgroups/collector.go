package cgroups

import (
	"context"
)

type Collector interface {
	Controller() string
	Collect(ctx context.Context, slice *Slice) (bool, error)
}
