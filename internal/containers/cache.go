package containers

import (
	"context"

	lru "github.com/hashicorp/golang-lru/v2"
)

type cachingResolver struct {
	cache    *lru.Cache[string, Container]
	resolver Resolver
}

var _ Resolver = &cachingResolver{}

func newCachingResolver(resolver Resolver) Resolver {
	cache, err := lru.New[string, Container](10)
	if err != nil {
		panic(err)
	}
	return &cachingResolver{
		cache:    cache,
		resolver: resolver,
	}
}

func (r *cachingResolver) Resolve(ctx context.Context, id string) (Container, error) {
	if container, ok := r.cache.Get(id); ok {
		return container, nil
	}

	container, err := r.resolver.Resolve(ctx, id)
	if err != nil {
		return Container{}, err
	}

	// XXX(konishchev): HERE
	r.cache.Add(id, container)
	return container, nil
}

func (r *cachingResolver) Close() error {
	return r.resolver.Close()
}
