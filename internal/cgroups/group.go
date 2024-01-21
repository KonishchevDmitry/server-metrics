package cgroups

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"syscall"

	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

const rootPath = "/sys/fs/cgroup"

type Group struct {
	Name string
}

func NewGroup(name string) *Group {
	return &Group{Name: name}
}

func (g *Group) IsRoot() bool {
	return g.Name == "/"
}

func (g *Group) IsExist() (bool, error) {
	return isExist(g.Path())
}

func (g *Group) Path() string {
	return path.Join(rootPath, g.Name)
}

func (g *Group) Child(name string) *Group {
	return NewGroup(path.Join(g.Name, name))
}

func (g *Group) Children() ([]*Group, bool, error) {
	files, exists, err := g.list()
	if err != nil || !exists {
		return nil, exists, err
	}

	var children []*Group

	for _, file := range files {
		if file.IsDir() {
			children = append(children, g.Child(file.Name()))
		}
	}

	return children, true, nil
}

func (g *Group) PIDs() ([]int, bool, error) {
	var pids []int

	if exists, err := g.ReadProperty("cgroup.procs", func(file io.Reader) error {
		return util.ParseFile(file, func(line string) error {
			pid, err := strconv.ParseInt(line, 10, 32)
			if err != nil || pid <= 0 {
				return fmt.Errorf("PID is expected, but got %q line", line)
			}
			pids = append(pids, int(pid))
			return nil
		})
	}); err != nil {
		if errors.Is(err, syscall.EOPNOTSUPP) {
			// cgroup.type == threaded
			return nil, true, nil
		}
		return nil, false, err
	} else if !exists {
		return nil, false, nil
	}

	return pids, true, nil
}

func (g *Group) HasProcesses() (bool, bool, error) {
	pids, exists, err := g.PIDs()
	return len(pids) != 0, exists, err
}

func (g *Group) ReadProperty(name string, reader func(file io.Reader) error) (bool, error) {
	groupPath := g.Path()
	propertyPath := path.Join(groupPath, name)

	if err := util.ReadFile(propertyPath, reader); err == nil {
		return true, nil
	} else if err := mapReadError(err); err != nil {
		return false, err
	}

	// Property file is missing. Before returning a misconfiguration error, check it for the following possible races:
	// * Group is deleting
	// * Group is creating and in process of configuration
	return false, util.RetryRace(fmt.Errorf("%q is missing", propertyPath), func() (bool, error) {
		if exists, err := isExist(groupPath); err != nil || !exists {
			return !exists, err
		}
		return isExist(propertyPath)
	})
}

func (g *Group) list() ([]os.DirEntry, bool, error) {
	files, err := os.ReadDir(g.Path())
	if err != nil {
		return nil, false, mapReadError(err)
	}
	return files, true, nil
}

func isExist(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return false, mapReadError(err)
	}
	return true, nil
}

func mapReadError(err error) error {
	switch {
	// The file is missing
	case errors.Is(err, syscall.ENOENT):
		return nil
	// The file has been deleted during reading
	case errors.Is(err, syscall.ENODEV):
		return nil
	default:
		return err
	}
}
