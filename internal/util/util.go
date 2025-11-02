package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func ReadFileReturning[T any](path string, reader func(file io.Reader) (T, error)) (T, error) {
	var result T

	if err := ReadFile(path, func(file io.Reader) error {
		var err error
		result, err = reader(file)
		return err
	}); err != nil {
		return result, err
	}

	return result, nil
}

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
		err = fmt.Errorf("Failed to read %q: %w", path, err)
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

func Uptime() time.Duration {
	var timespec unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &timespec); err != nil {
		panic(err)
	}
	return time.Duration(timespec.Nano()) * time.Nanosecond
}
