package util

import (
	"bufio"
	"io"
	"os"
	"strings"

	"golang.org/x/xerrors"
)

func ReadFile(path string, reader func(file io.Reader) error) (resErr error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil && resErr == nil {
			resErr = err
		}
	}()

	err = reader(file)
	if err != nil {
		err = xerrors.Errorf("Failed to read %q: %w", path, err)
	}

	return err
}

func ParseFile(reader io.Reader, parser func(line string) error) error {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		if err := parser(line); err != nil {
			return err
		}
	}

	return scanner.Err()
}
