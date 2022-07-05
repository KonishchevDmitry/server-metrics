package classifier

import (
	"context"
	"path"
	"strconv"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/docker"
	"github.com/KonishchevDmitry/server-metrics/internal/users"
)

type Classifier struct {
	users  users.Resolver
	docker docker.Resolver
}

func New(users users.Resolver, docker docker.Resolver) *Classifier {
	return &Classifier{
		users:  users,
		docker: docker,
	}
}

func (c *Classifier) ClassifySlice(ctx context.Context, name string) (
	retService string, retTotalCollection bool, retClassified bool, retErr error,
) {
	classify := func(service string, totalCollection bool) (string, bool, bool, error) {
		return service, totalCollection, true, nil
	}

	name = strings.ReplaceAll(name, `\x2d`, `-`)

	switch name {
	case "/":
		return classify("kernel", false)
	case "/init.scope":
		return classify("init", false)
	}

	parent, child := path.Split(name)
	parent = path.Clean(parent)

	if (strings.HasSuffix(child, ".mount") || strings.HasSuffix(child, ".socket")) &&
		(parent == "/" || parent == "/system.slice") {
		return classify(child, false)
	}

	trimChild := func(prefix string, suffix string) string {
		return child[len(prefix) : len(child)-len(suffix)]
	}

	switch parent {
	case "/system.slice":
		const serviceSuffix = ".service"

		const serviceGroupPrefix = "system-"
		const serviceGroupSuffix = ".slice"

		const dockerPrefix = "docker-"
		const dockerSuffix = ".scope"

		const dockerBuilderPrefix = "system.slice:docker:"

		switch {
		case strings.HasSuffix(child, serviceSuffix):
			return classify(trimChild("", serviceSuffix), false)

		case strings.HasPrefix(child, serviceGroupPrefix) && strings.HasSuffix(child, serviceGroupSuffix):
			return classify(trimChild(serviceGroupPrefix, serviceGroupSuffix), true)

		case strings.HasPrefix(child, dockerPrefix) && strings.HasSuffix(child, dockerSuffix):
			containerID := trimChild(dockerPrefix, dockerSuffix)

			container, err := c.docker.Resolve(ctx, containerID)
			if err != nil {
				return "", false, false, err
			}

			service := container.Name
			if container.Temporary {
				service = "docker-containers"
			}

			return classify(service, false)

		case strings.HasPrefix(child, dockerBuilderPrefix) && path.Ext(child[len(dockerBuilderPrefix):]) == "":
			return classify("docker-builder", false)
		}

	case "/user.slice":
		const prefix = "user-"
		const suffix = ".slice"

		if strings.HasPrefix(child, prefix) && strings.HasSuffix(child, suffix) {
			if uid, err := strconv.Atoi(trimChild(prefix, suffix)); err == nil && uid >= 0 {
				userName, err := c.users.Resolve(uid)
				if err != nil {
					err = xerrors.Errorf("Unable to resolve %d user ID: %w", uid, err)
					return "", false, false, err
				}
				return classify(userName, true)
			}
		}
	}

	return "", false, false, nil
}
