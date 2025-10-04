package containers

import (
	"context"
)

type Container struct {
	Name      string
	Temporary bool // Temporary containers have auto-generated names
}

type Resolver interface {
	Resolve(ctx context.Context, id string) (Container, error)
	Close() error
}
