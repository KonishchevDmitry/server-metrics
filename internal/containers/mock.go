package containers

import (
	"context"
	"fmt"
)

type resolverMock struct {
	containers map[string]Container
}

func NewResolverMock(containers map[string]Container) Resolver {
	return &resolverMock{containers: containers}
}

func (r *resolverMock) Resolve(ctx context.Context, id string) (Container, error) {
	container, ok := r.containers[id]
	if !ok {
		return Container{}, fmt.Errorf("Invalid container ID: %q", id)
	}
	return container, nil
}

func (r *resolverMock) Close() error {
	return nil
}
