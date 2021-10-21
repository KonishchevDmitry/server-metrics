package cgroupsutil

import (
	"io"
	"strconv"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

func ReadNamedStat(group *cgroups.Group, name string) (map[string]Stat, bool, error) {
	var stats map[string]Stat
	exists, err := group.ReadProperty(name, func(file io.Reader) (err error) {
		stats, err = parseNamedStat(name, file)
		return
	})
	return stats, exists, err
}

func parseNamedStat(propertyName string, reader io.Reader) (map[string]Stat, error) {
	stats := make(map[string]Stat)

	if err := util.ParseFile(reader, func(line string) error {
		tokens := strings.Split(line, " ")
		if len(tokens) < 2 {
			return xerrors.Errorf("Got an unexpected stat line: %q", line)
		}

		name := tokens[0]
		if _, ok := stats[name]; ok {
			return xerrors.Errorf("Got a duplicated %q name", name)
		}

		stat := make(map[string]int64)

		for _, token := range tokens[1:] {
			tokens := strings.Split(token, "=")
			if len(tokens) != 2 {
				return xerrors.Errorf("Got an unexpected stat line: %q", line)
			}

			value, err := strconv.ParseInt(tokens[1], 10, 64)
			if err != nil {
				return xerrors.Errorf("Got an unexpected stat line: %q", line)
			}

			key := tokens[0]
			if _, ok := stat[key]; ok {
				return xerrors.Errorf("Got a duplicated %q key", key)
			}

			stat[key] = value
		}

		stats[name] = Stat{
			name: propertyName,
			stat: stat,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return stats, nil
}
