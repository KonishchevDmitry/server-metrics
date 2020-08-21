package cgroups

import (
	"io/ioutil"
	"os"
)

type slice struct {
	name     string
	path     string
	children []*slice
}

func listSlice(path string) ([]string, bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}

	var children []string

	for _, file := range files {
		if file.IsDir() {
			children = append(children, file.Name())
		}
	}

	return children, true, nil
}
