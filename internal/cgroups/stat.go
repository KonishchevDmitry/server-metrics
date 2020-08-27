package cgroups

import (
	"io"
	"path"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
)

type Stat struct {
	name string
	stat map[string]int64
}

func (s *Stat) Get(name string) (int64, error) {
	value, ok := s.stat[name]
	if !ok {
		return 0, xerrors.Errorf("%q entry of %s is missing", name, s.name)
	}
	return value, nil
}

func ReadStat(slice *Slice, name string) (Stat, bool, error) {
	stat := Stat{name: name}

	exists, err := ReadFile(path.Join(slice.Path, name), func(file io.Reader) (exists bool, err error) {
		stat.stat, exists, err = parseStat(file)
		return
	})

	return stat, exists, err
}

func parseStat(reader io.Reader) (map[string]int64, bool, error) {
	stat := make(map[string]int64)

	exists, err := ParseFile(reader, func(line string) error {
		tokens := strings.Split(line, " ")
		if len(tokens) != 2 {
			return xerrors.Errorf("Got an unexpected stat line: %q", line)
		}

		name := tokens[0]
		if _, ok := stat[name]; ok {
			return xerrors.Errorf("Got a duplicated %q key", name)
		}

		value, err := strconv.ParseInt(tokens[1], 10, 64)
		if err != nil {
			return xerrors.Errorf("Got an unexpected stat line: %q", line)
		}

		stat[name] = value
		return nil
	})

	return stat, exists, err
}
