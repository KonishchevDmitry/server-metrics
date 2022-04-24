package cgroups

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
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

func (g *Group) HasProcesses() (bool, bool, error) {
	var hasProcesses bool

	if exists, err := g.ReadProperty("cgroup.procs", func(file io.Reader) error {
		return util.ParseFile(file, func(line string) error {
			if _, err := strconv.ParseInt(line, 10, 32); err != nil {
				return fmt.Errorf("PID is expected, but got %q line", line)
			}
			hasProcesses = true
			return nil
		})
	}); err != nil {
		if errors.Is(err, syscall.EOPNOTSUPP) {
			// cgroup.type == threaded
			return false, true, nil
		}
		return false, false, err
	} else if !exists {
		return false, false, nil
	}

	return hasProcesses, true, nil
}

func (g *Group) ReadProperty(name string, reader func(file io.Reader) error) (bool, error) {
	propertyPath := path.Join(g.Path(), name)

	if err := util.ReadFile(propertyPath, reader); err == nil {
		return true, nil
	} else if err := mapReadError(err); err != nil {
		return false, err
	}

	// Property file is missing. Ensure that this is due to missing group and not due to group misconfiguration.

	files, exists, err := g.list()
	if err != nil || !exists {
		return exists, err
	}

	// Group exists

	for _, file := range files {
		if !file.IsDir() && file.Name() == name {
			// Property file also exists. Considering it as a race.
			return false, nil
		}
	}

	return false, fmt.Errorf("%q is missing", propertyPath)
}

func (g *Group) list() ([]fs.FileInfo, bool, error) {
	files, err := ioutil.ReadDir(g.Path())
	if err != nil {
		return nil, false, mapReadError(err)
	}
	return files, true, nil
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
