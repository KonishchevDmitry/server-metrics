package containers

import (
	"context"
	"strings"
	"sync"

	"github.com/docker/docker/client"
	lru "github.com/hashicorp/golang-lru/v2"
)

type dockerResolver struct {
	lock   sync.Mutex
	cache  *lru.Cache[string, Container]
	client *client.Client
}

var _ Resolver = &dockerResolver{}

func NewDockerResolver() Resolver {
	cache, err := lru.New[string, Container](10)
	if err != nil {
		panic(err)
	}
	return &dockerResolver{cache: cache}
}

func (r *dockerResolver) Resolve(ctx context.Context, id string) (Container, error) {
	if container, ok := r.cache.Get(id); ok {
		return container, nil
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

func (r *dockerResolver) getClient() (*client.Client, error) {
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

func (r *dockerResolver) Close() error {
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
