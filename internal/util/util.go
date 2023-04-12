package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type WaitGroup struct {
	group sync.WaitGroup
}

func (g *WaitGroup) Run(runner func()) {
	g.group.Add(1)
	go func() {
		defer g.group.Done()
		runner()
	}()
}

func (g *WaitGroup) Wait() {
	g.group.Wait()
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

func RetryRace(failure error, retry func() (bool, error)) error {
	period := 10 * time.Millisecond
	deadline := time.Now().Add(time.Second)

	for time.Until(deadline) >= period {
		time.Sleep(period)

		if ok, err := retry(); err != nil || ok {
			return err
		}
	}

	return failure
}

func Uptime() time.Duration {
	var timespec unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &timespec); err != nil {
		panic(err)
	}
	return time.Duration(timespec.Nano()) * time.Nanosecond
}
