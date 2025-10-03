package classifier

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/mo"
	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/containers"
	"github.com/KonishchevDmitry/server-metrics/internal/users"
)

type Classification struct {
	Service        string
	TotalExcluding mo.Option[[]string]
}

type Classifier struct {
	users  users.Resolver
	docker containers.Resolver
}

func New(users users.Resolver, docker containers.Resolver) *Classifier {
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
			// The group contains:
			// * user@1000.service - systemd user session
			// * session-*.scope - each ssh/mosh connection is assigned to a session
			return system.classifyTotal(userName+"/sessions", systemdUserServiceName)
		}

		user := classifyContext{slice: "app", prefix: userName + "/"}

		// /user.slice/user-1000.slice/*
		if match[3] == "" {
			if child != systemdUserServiceName {
				return Classification{}, false, nil
			}

			// user@1000.service contains:
			// * init.scope - systemd
			// * app.slice - services
			// * tmux-spawn-2c342f76-8d5b-4046-919e-b14ed5265ad2.scope – each tmux window runs in a separate scope
			//
			// user@1000.service is expected to have no processes, but when user session is being started systemd is
			// placed here first and only then is being moved to init.scope

			return Classification{
				Service:        userName,
				TotalExcluding: mo.Some([]string{"app.slice", "init.scope"}),
			}, true, nil
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
		// /user.slice/user-1000.slice/user@1000.service/app.slice/snap.*.*-*.scope
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

const uuidRegex = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

var (
	dbusActivationRegex = regexp.MustCompile(`^\d+\.\d+-(.+)\.slice$`)
	snapScopeNameRegex  = regexp.MustCompile(`^snap\.[^.]+\.([^.]+)-` + uuidRegex + `\.scope$`)
)

func (c *Classifier) classifyServiceSliceChild(ctx context.Context, context classifyContext, name string) (
	Classification, bool, error,
) {
	// DBus creates a unique unit for each service activation:
	// /system.slice/system-dbus\x2d:1.4\x2dorg.fedoraproject.SetroubleshootPrivileged.slice/dbus-:1.4-org.fedoraproject.SetroubleshootPrivileged@21.service
	dbusActivationPrefix := fmt.Sprintf(`%s-dbus-:`, context.slice)
	if strings.HasPrefix(name, dbusActivationPrefix) {
		if match := dbusActivationRegex.FindStringSubmatch(name[len(dbusActivationPrefix):]); len(match) != 0 {
			return context.classifyTotal("dbus:" + match[1])
		}
	}

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

	if match := snapScopeNameRegex.FindStringSubmatch(name); len(match) != 0 {
		return context.classify(match[1])
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
		Service:        c.prefix + service,
		TotalExcluding: mo.Some(exclude),
	}, true, nil
}

func trim(prefix string, name string, suffix string) string {
	return name[len(prefix) : len(name)-len(suffix)]
}
