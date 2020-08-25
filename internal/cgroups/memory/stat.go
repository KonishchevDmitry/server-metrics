package memory

import (
	"io"
	"strconv"
	"strings"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"

	"golang.org/x/xerrors"
)

func readStat(path string) (stat map[string]int64, exists bool, err error) {
	exists, err = cgroups.ReadFile(path, func(file io.Reader) (exists bool, err error) {
		stat, exists, err = parseStat(file)
		return
	})
	return
}

func parseStat(reader io.Reader) (map[string]int64, bool, error) {
	stat := make(map[string]int64)

	exists, err := cgroups.ParseFile(reader, func(line string) error {
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
