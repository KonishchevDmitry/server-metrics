package meminfo

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

func readMeminfo() (map[string]int64, error) {
	return util.ReadFileReturning("/proc/meminfo", parseMeminfo)
}

func parseMeminfo(reader io.Reader) (map[string]int64, error) {
	meminfo := make(map[string]int64)

	if err := util.ParseFile(reader, func(line string) error {
		lineTokens := strings.SplitN(line, ":", 2)
		if len(lineTokens) != 2 {
			return fmt.Errorf("got an unexpected meminfo line: %q", line)
		}

		name := strings.TrimSpace(lineTokens[0])
		valueTokens := strings.Fields(lineTokens[1])

		if count := len(valueTokens); count != 1 && count != 2 {
			return fmt.Errorf("got an unexpected meminfo line: %q", line)
		}

		value, err := strconv.ParseInt(valueTokens[0], 10, 64)
		if len(valueTokens) > 1 {
			switch valueTokens[1] {
			case "kB":
				value *= 1024
			default:
				err = errors.New("unknown units")
			}
		}
		if err != nil {
			return fmt.Errorf("got an unexpected meminfo line: %q", line)
		}

		if _, ok := meminfo[name]; ok {
			return fmt.Errorf("got a duplicated counter: %q", name)
		}
		meminfo[name] = value

		return nil
	}); err != nil {
		return nil, err
	}

	return meminfo, nil
}
