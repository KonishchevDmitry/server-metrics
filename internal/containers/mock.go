package containers

import (
	"context"
	"errors"
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
		return Container{}, errors.New("Invalid container ID")
	}
	return container, nil
}

func (r *resolverMock) Close() error {
	return nil
}
