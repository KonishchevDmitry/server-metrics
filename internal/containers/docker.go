package containers

import (
	"context"
	"strings"
	"sync"

	"github.com/docker/docker/client"
)

type dockerResolver struct {
	lock   sync.Mutex
	client *client.Client
}

var _ Resolver = &dockerResolver{}

func NewDockerResolver() Resolver {
	return newCachingResolver(&dockerResolver{})
}

func (r *dockerResolver) Resolve(ctx context.Context, id string) (Container, error) {
	cli, err := r.getClient()
	if err != nil {
		return Container{}, err
	}

	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return Container{}, err
	}

	var temporary bool
	if hostConfig := info.HostConfig; hostConfig != nil {
		temporary = hostConfig.AutoRemove
	}

	return Container{
		Name:      strings.TrimLeft(info.Name, "/"),
		Temporary: temporary,
	}, nil
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
