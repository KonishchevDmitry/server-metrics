package docker

import (
	"context"

	"golang.org/x/xerrors"
)

type fakeResolver struct {
	containers map[string]Container
}

func NewFakeResolver(containers map[string]Container) Resolver {
	return &fakeResolver{containers: containers}
}

func (r *fakeResolver) Resolve(ctx context.Context, id string) (Container, error) {
	container, ok := r.containers[id]
	if !ok {
		return Container{}, xerrors.New("Invalid container ID")
	}
	return container, nil
}

func (r *fakeResolver) Close() error {
	return nil
}
