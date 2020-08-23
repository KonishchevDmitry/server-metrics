package cgroups

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"

	"golang.org/x/xerrors"
)

type Slice struct {
	Name     string
	Path     string
	Children []*Slice
}

func (s *Slice) HasTasks(ctx context.Context) (bool, error) {
	var hasTasks bool

	if exists, err := ReadFile(path.Join(s.Path, "tasks"), func(file io.Reader) (bool, error) {
		return ParseFile(file, func(line string) error {
			if _, err := strconv.ParseInt(line, 10, 32); err != nil {
				return xerrors.Errorf("Task ID is expected, but got %q line", line)
			}
			hasTasks = true
			return nil
		})
	}); err != nil {
		return false, err
	} else if !exists {
		logging.L(ctx).Debugf("%q has been deleted during discovering.", s.Path)
		return false, nil
	}

	return hasTasks, nil
}

func ListSlice(path string) ([]string, bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return nil, false, err
	}

	var children []string

	for _, file := range files {
		if file.IsDir() {
			children = append(children, file.Name())
		}
	}

	return children, true, nil
}
