package slab

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

func readSlabInfo() ([]slabInfo, error) {
	path := "/proc/slabinfo"

	var slabs []slabInfo
	if err := util.ReadFile(path, func(file io.Reader) error {
		var err error
		slabs, err = parseSlabInfo(file)
		return err
	}); err != nil {
		return nil, err
	}

	return slabs, nil
}

func parseSlabInfo(reader io.Reader) ([]slabInfo, error) {
	var (
		lineIndex int
		header    slabInfoHeader
		slabs     []slabInfo
	)

	if err := util.ParseFile(reader, func(line string) error {
		var err error

		switch lineIndex {
		case 0:
			if !strings.HasPrefix(line, "slabinfo - version:") {
				return fmt.Errorf("got an unexpected first line: %q", line)
			}

		case 1:
			headerPrefix := "# "
			if !strings.HasPrefix(line, headerPrefix) {
				return fmt.Errorf("got an unexpected second line: %q", line)
			}

			header, err = parseSlabInfoHeader(line[len(headerPrefix):])
			if err != nil {
				return fmt.Errorf("failed to parse the header: %w", err)
			}

		default:
			slab, err := parseSlabInfoStatLine(header, line)
			if err != nil {
				return fmt.Errorf("failed to parse stat line %q: %w", line, err)
			}
			slabs = append(slabs, slab)
		}

		lineIndex++
		return nil
	}); err != nil {
		return nil, err
	} else if len(slabs) == 0 {
		return nil, errors.New("the file is empty")
	}

	return slabs, nil
}

type slabInfoHeader struct {
	columns            int
	numSlabsColumn     int
	pagesPerSlabColumn int
}

func parseSlabInfoHeader(line string) (slabInfoHeader, error) {
	columns := splitSlabInfoLine(line)
	if len(columns) < 2 || columns[0] != "name" {
		return slabInfoHeader{}, fmt.Errorf("unexpected header: %q", line)
	}

	header := slabInfoHeader{
		columns: len(columns),
	}
	mapping := map[string]*int{
		"<pagesperslab>": &header.pagesPerSlabColumn,
		"<num_slabs>":    &header.numSlabsColumn,
	}

	for column := 1; column < len(columns); column++ {
		name := columns[column]

		if index, ok := mapping[name]; ok {
			if *index != 0 {
				return slabInfoHeader{}, fmt.Errorf("got duplicated %s column", name)
			}
			*index = column
		}
	}

	for name, index := range mapping {
		if *index == 0 {
			return slabInfoHeader{}, fmt.Errorf("column %s is missing", name)
		}
	}

	return header, nil
}

type slabInfo struct {
	name string
	size int64
}

var pageSize = int64(os.Getpagesize())

func parseSlabInfoStatLine(header slabInfoHeader, line string) (slabInfo, error) {
	columns := splitSlabInfoLine(line)
	if len(columns) != header.columns {
		return slabInfo{}, fmt.Errorf("got %d columns when %d is expected", len(columns), header.columns)
	}

	parse := func(name string, index int, strictlyPositive bool) (int64, error) {
		value, err := strconv.ParseInt(columns[index], 10, 64)
		if err != nil || value < 0 || strictlyPositive && value == 0 {
			return 0, fmt.Errorf("invalid <%s> value", name)
		}
		return value, nil
	}

	count, err := parse("num_slabs", header.numSlabsColumn, false)
	if err != nil {
		return slabInfo{}, err
	}

	pagesPerSlab, err := parse("pagesperslab", header.pagesPerSlabColumn, true)
	if err != nil {
		return slabInfo{}, err
	}

	return slabInfo{
		name: columns[0],
		size: count * pagesPerSlab * pageSize,
	}, nil
}

var slabInfoSeparator = regexp.MustCompile(` +`)

func splitSlabInfoLine(line string) []string {
	return slabInfoSeparator.Split(line, -1)
}
