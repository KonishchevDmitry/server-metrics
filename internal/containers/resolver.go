package containers

import (
	"context"
)

type Container struct {
	Name      string
	Temporary bool
}

type Resolver interface {
	Resolve(ctx context.Context, id string) (Container, error)
	Close() error
}
