package classifier

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/docker"
	"github.com/KonishchevDmitry/server-metrics/internal/users"
)

type Classification struct {
	Service              string
	SystemdUserRoot      bool
	TotalCollection      bool
	TotalExcludeChildren []string
}

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

var systemSlicePathRegex = regexp.MustCompile(`^/system\.slice(/system-[^/]+.slice)?$`)
var userSliceNameRegex = regexp.MustCompile(`^user-(\d+)\.slice$`)
var userSlicePathRegex = regexp.MustCompile(`^/user\.slice(/user-(\d+)\.slice(/user@\d+\.service(/(?:app|session)\.slice(?:/(?:app|session)-[^/]+.slice)?)?)?)?$`)

func (c *Classifier) ClassifySlice(ctx context.Context, name string) (Classification, bool, error) {
	name = strings.ReplaceAll(name, `\x2d`, `-`)
	system := classifyContext{slice: "system"}

	if name == "/" {
		return system.classify("kernel")
	}

	parent, child := path.Split(name)
	parent = path.Clean(parent)

	if parent == "/" {
		if child == "init.scope" {
			return system.classify("init")
		}
		return c.classifySupplementaryChild(system, child)
	} else if match := systemSlicePathRegex.FindStringSubmatch(parent); len(match) != 0 {
		// /system.slice/*
		// /system.slice/system-*.slice/*
		if classification, ok, err := c.classifyServiceSliceChild(ctx, system, child); err != nil || ok {
			return classification, ok, err
		}
		return c.classifySupplementaryChild(system, child)
	} else if match := userSlicePathRegex.FindStringSubmatch(parent); len(match) != 0 {
		uidString := match[2]

		// /user.slice/*
		if match[1] == "" {
			nameMatch := userSliceNameRegex.FindStringSubmatch(child)
			if len(nameMatch) == 0 {
				return Classification{}, false, nil
			}
			uidString = nameMatch[1]
		}

		uid, err := strconv.Atoi(uidString)
		if err != nil {
			return Classification{}, false, err
		}

		userName, err := c.users.Resolve(uid)
		if err != nil {
			err = xerrors.Errorf("Unable to resolve %d user ID: %w", uid, err)
			return Classification{}, false, err
		}

		systemdUserServiceName := fmt.Sprintf("user@%d.service", uid)

		// /user.slice/user-1000.slice
		if match[1] == "" {
			return system.classifyTotal(userName, systemdUserServiceName)
		}

		user := classifyContext{slice: "app", prefix: userName + "/"}

		// /user.slice/user-1000.slice/*
		if match[3] == "" {
			if child == systemdUserServiceName {
				return Classification{
					Service:         userName,
					SystemdUserRoot: true,
				}, true, nil
			}
			return Classification{}, false, nil
		}

		// /user.slice/user-1000.slice/user@1000.service/*
		if match[4] == "" {
			if child == "init.scope" {
				return user.classify("init")
			}
			return Classification{}, false, nil
		}

		// /user.slice/user-1000.slice/user@1000.service/app.slice/*
		// /user.slice/user-1000.slice/user@1000.service/app.slice/app-*.slice/*
		// /user.slice/user-1000.slice/user@1000.service/session.slice/*
		// /user.slice/user-1000.slice/user@1000.service/session.slice/session-*.slice/*
		if classification, ok, err := c.classifyServiceSliceChild(ctx, user, child); err != nil || ok {
			return classification, ok, err
		}
		return c.classifySupplementaryChild(user, child)
	} else {
		return Classification{}, false, nil
	}
}

func (c *Classifier) classifySupplementaryChild(context classifyContext, name string) (
	Classification, bool, error,
) {
	if strings.HasSuffix(name, ".mount") || strings.HasSuffix(name, ".socket") {
		return context.classify(name)
	}
	return Classification{}, false, nil
}

func (c *Classifier) classifyServiceSliceChild(ctx context.Context, context classifyContext, name string) (
	Classification, bool, error,
) {
	serviceSuffix := ".service"
	if strings.HasSuffix(name, serviceSuffix) {
		service := trim("", name, serviceSuffix)

		// It has a non-standard cgroups configuration
		if service == "systemd-udevd" {
			return context.classifyTotal(service)
		}

		return context.classify(service)
	}

	dockerPrefix, dockerSuffix := "docker-", ".scope"
	if strings.HasPrefix(name, dockerPrefix) && strings.HasSuffix(name, dockerSuffix) {
		containerID := trim(dockerPrefix, name, dockerSuffix)

		container, err := c.docker.Resolve(ctx, containerID)
		if err != nil {
			return Classification{}, false, err
		}

		service := container.Name
		if container.Temporary {
			service = "docker-containers"
		}

		return context.classify(service)
	}

	dockerBuilderPrefix := fmt.Sprintf("%s.slice:docker:", context.slice)
	if strings.HasPrefix(name, dockerBuilderPrefix) && path.Ext(name[len(dockerBuilderPrefix):]) == "" {
		return context.classify("docker-builder")
	}

	return Classification{}, false, nil
}

type classifyContext struct {
	slice  string
	prefix string
}

func (c classifyContext) classify(service string) (Classification, bool, error) {
	return Classification{Service: c.prefix + service}, true, nil
}

func (c classifyContext) classifyTotal(service string, exclude ...string) (Classification, bool, error) {
	return Classification{
		Service:              c.prefix + service,
		TotalCollection:      true,
		TotalExcludeChildren: exclude,
	}, true, nil
}

func trim(prefix string, name string, suffix string) string {
	return name[len(prefix) : len(name)-len(suffix)]
}
