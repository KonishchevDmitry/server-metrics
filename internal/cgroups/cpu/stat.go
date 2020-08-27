package cpu

import (
	"io"
	"path"
	"strconv"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
)

type stat struct {
	user   uint64
	system uint64
}

func readStat(slice *cgroups.Slice) (stat stat, exists bool, err error) {
	statPath := path.Join(slice.Path, "cpuacct.usage_all")

	exists, err = cgroups.ReadFile(statPath, func(file io.Reader) (exists bool, err error) {
		stat, exists, err = parseStat(file)
		return
	})

	return
}

func parseStat(reader io.Reader) (stat, bool, error) {
	var stat stat

	header := true
	parse := func(line string) bool {
		if header {
			header = false
			return line == "cpu user system"
		}

		tokens := strings.Split(line, " ")
		if len(tokens) != 3 {
			return false
		}

		user, err := strconv.ParseInt(tokens[1], 10, 64)
		if err != nil || user < 0 {
			return false
		}

		system, err := strconv.ParseInt(tokens[2], 10, 64)
		if err != nil || system < 0 {
			return false
		}

		stat.user += uint64(user)
		stat.system += uint64(system)

		return true
	}

	exists, err := cgroups.ParseFile(reader, func(line string) error {
		if !parse(line) {
			return xerrors.Errorf("Got an unexpected cpuacct stat line: %q", line)
		}
		return nil
	})

	return stat, exists, err
}
