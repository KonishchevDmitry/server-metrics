package docker

import (
	"context"
	"strings"
	"sync"

	"github.com/docker/docker/client"
	lru "github.com/hashicorp/golang-lru"
)

type Resolver interface {
	Resolve(ctx context.Context, id string) (Container, error)
	Close() error
}

type Container struct {
	Name      string
	Temporary bool
}

type resolver struct {
	lock   sync.Mutex
	cache  *lru.Cache
	client *client.Client
}

var _ Resolver = &resolver{}

func NewResolver() Resolver {
	cache, err := lru.New(10)
	if err != nil {
		panic(err)
	}
	return &resolver{cache: cache}
}

func (r *resolver) Resolve(ctx context.Context, id string) (Container, error) {
	if container, ok := r.cache.Get(id); ok {
		return container.(Container), nil
	}

	cli, err := r.getClient()
	if err != nil {
		return Container{}, err
	}

	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return Container{}, err
	}

	container := Container{
		Name:      strings.TrimLeft(info.Name, "/"),
		Temporary: info.HostConfig.AutoRemove,
	}
	r.cache.Add(id, container)

	return container, nil
}

func (r *resolver) getClient() (*client.Client, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.client == nil {
		var err error

		r.client, err = client.NewClientWithOpts()
		if err != nil {
			return nil, err
		}
	}

	return r.client, nil
}

func (r *resolver) Close() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.client != nil {
		if err := r.client.Close(); err != nil {
			return err
		}
		r.client = nil
	}

	return nil
}
