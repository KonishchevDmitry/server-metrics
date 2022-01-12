package classifier

import (
	"context"
	"path"
	"strings"

	"github.com/KonishchevDmitry/server-metrics/internal/docker"
)

type Classifier struct {
	docker docker.Resolver
}

func New(docker docker.Resolver) *Classifier {
	return &Classifier{docker: docker}
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
	case "/user.slice":
		return classify("user", true)
	}

	parent, child := path.Split(name)
	parent = path.Clean(parent)

	if (strings.HasSuffix(child, ".mount") || strings.HasSuffix(child, ".socket")) &&
		(parent == "/" || parent == "/system.slice") {
		return classify(child, false)
	}

	if parent == "/system.slice" {
		trimChild := func(prefix string, suffix string) string {
			return child[len(prefix) : len(child)-len(suffix)]
		}

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
	}

	return "", false, false, nil
}
