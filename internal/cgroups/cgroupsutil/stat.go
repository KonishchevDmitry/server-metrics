package cgroupsutil

import (
	"io"
	"strconv"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/util"
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

func ReadStat(group *cgroups.Group, name string) (Stat, bool, error) {
	stat := Stat{name: name}

	exists, err := group.ReadProperty(name, func(file io.Reader) (err error) {
		stat.stat, err = parseStat(file)
		return
	})

	return stat, exists, err
}

func parseStat(reader io.Reader) (map[string]int64, error) {
	stat := make(map[string]int64)

	if err := util.ParseFile(reader, func(line string) error {
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
	}); err != nil {
		return nil, err
	}

	return stat, nil
}
