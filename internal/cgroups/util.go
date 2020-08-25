package cgroups

import (
	"bufio"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/xerrors"
)

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
