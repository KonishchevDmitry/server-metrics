package containers

import (
	"context"
	"sync"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/samber/mo"
)

type podmanResolver struct {
	lock          sync.Mutex
	clientContext mo.Option[context.Context]
}

var _ Resolver = &podmanResolver{}

func NewPodmanResolver() Resolver {
	return newCachingResolver(&podmanResolver{})
}

func (r *podmanResolver) Resolve(_ context.Context, id string) (Container, error) {
	clientContext, err := r.getClientContext()
	if err != nil {
		return Container{}, err
	}

	info, err := containers.Inspect(clientContext, id, nil) //nolint:contextcheck
	if err != nil {
		return Container{}, err
	}

	var temporary bool
	if hostConfig := info.HostConfig; hostConfig != nil {
		temporary = hostConfig.AutoRemove
	}

	return Container{
		Name:      info.Name,
		Temporary: temporary,
	}, nil
}

func (r *podmanResolver) getClientContext() (context.Context, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	clientContext, ok := r.clientContext.Get()
	if ok {
		return clientContext, nil
	}

	clientContext, err := bindings.NewConnection(context.Background(), "unix:///run/podman/podman.sock")
	if err != nil {
		return nil, err
	}
	r.clientContext = mo.Some(clientContext)

	return clientContext, nil
}

func (r *podmanResolver) Close() error {
	return nil
}
