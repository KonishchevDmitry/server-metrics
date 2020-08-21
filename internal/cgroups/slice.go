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

type slice struct {
	name     string
	path     string
	children []*slice
}

func (s *slice) hasTasks(ctx context.Context) (bool, error) {
	var hasTasks bool

	if ok, err := readFile(path.Join(s.path, "tasks"), func(file io.Reader) (bool, error) {
		return parseFile(file, func(line string) error {
			if _, err := strconv.ParseInt(line, 10, 32); err != nil {
				return xerrors.Errorf("Task ID is expected, but got %q line", line)
			}
			hasTasks = true
			return nil
		})
	}); err != nil {
		return false, err
	} else if !ok {
		logging.L(ctx).Debugf("%q has been deleted during discovering.", s.path)
		return false, nil
	}

	return hasTasks, nil
}

func listSlice(path string) ([]string, bool, error) {
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
