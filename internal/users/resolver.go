package users

import (
	"errors"
	users "os/user"
	"strconv"
	"time"

	lru "github.com/hnlq715/golang-lru"
	"golang.org/x/xerrors"
)

type Resolver interface {
	Resolve(id int) (string, error)
}

type resolver struct {
	cache *lru.Cache
}

var _ Resolver = &resolver{}

func NewResolver() Resolver {
	cache, err := lru.NewWithExpire(10, time.Minute)
	if err != nil {
		panic(err)
	}
	return &resolver{cache: cache}
}

func (r *resolver) Resolve(id int) (string, error) {
	if name, ok := r.cache.Get(id); ok {
		return name.(string), nil
	}

	user, err := users.LookupId(strconv.Itoa(id))
	if err != nil {
		var unknownUserIDError users.UnknownUserIdError
		if errors.As(err, &unknownUserIDError) {
			err = xerrors.New("Invalid user ID")
		}
		return "", err
	}

	r.cache.Add(id, user.Username)
	return user.Username, nil
}
