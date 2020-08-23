package cgroups

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/xerrors"
)

func ReadStat(path string) (stat map[string]int64, exists bool, err error) {
	exists, err = ReadFile(path, func(file io.Reader) (ok bool, err error) {
		stat, ok, err = parseStat(file)
		return
	})
	return
}

func ReadFile(path string, reader func(file io.Reader) (bool, error)) (resExists bool, resErr error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return false, err
	}
	defer func() {
		if err := file.Close(); err != nil && resErr == nil {
			resErr = err
		}
	}()

	exists, err := reader(file)
	if err != nil {
		err = xerrors.Errorf("Failed to read %q: %w", path, err)
	}

	return exists, err
}

func ParseFile(reader io.Reader, parser func(line string) error) (bool, error) {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		if err := parser(line); err != nil {
			return false, err
		}
	}

	if err := scanner.Err(); err != nil {
		if xerrors.Is(err, syscall.ENODEV) {
			err = nil
		}
		return false, err
	}

	return true, nil
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
