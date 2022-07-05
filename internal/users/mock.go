package users

import (
	"golang.org/x/xerrors"
)

type resolverMock struct {
	users map[int]string
}

func NewResolverMock(users map[int]string) Resolver {
	return &resolverMock{users: users}
}

func (r *resolverMock) Resolve(id int) (string, error) {
	name, ok := r.users[id]
	if !ok {
		return "", xerrors.New("Invalid user ID")
	}
	return name, nil
}
