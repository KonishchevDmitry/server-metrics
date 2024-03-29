package cgroupsutil

import (
	"fmt"
	"io"
	"strconv"
	"strings"

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
		var nameRead bool

		stat := make(map[string]int64)
		lineTokens := strings.Split(line, " ")

		for _, lineToken := range lineTokens {
			tokens := strings.Split(lineToken, "=")

			if len(tokens) == 1 && len(stat) == 0 {
				name := tokens[0]
				nameRead = true

				if _, ok := stats[name]; ok {
					return fmt.Errorf("Got a duplicated %q name", name)
				}

				// io.stat may contain the following lines: "9:0 ". Don't know the real reason, but it might be an
				// artefact of IO accounting specific (per inode accounting, page cache issues - see the docs for
				// details). So just skip such lines.
				if len(lineTokens) == 1 {
					return nil
				}

				stats[name] = Stat{
					name: propertyName,
					stat: stat,
				}
				continue
			}

			if len(tokens) != 2 || !nameRead {
				return fmt.Errorf("Got an unexpected stat line: %q", line)
			}

			value, err := strconv.ParseInt(tokens[1], 10, 64)
			if err != nil {
				return fmt.Errorf("Got an unexpected stat line: %q", line)
			}

			key := tokens[0]
			if _, ok := stat[key]; ok {
				return fmt.Errorf("Got a duplicated %q key", key)
			}

			stat[key] = value
		}

		if len(stat) == 0 {
			return fmt.Errorf("Got an unexpected stat line: %q", line)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return stats, nil
}
