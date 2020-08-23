package blkio

import (
	"io"
	"strconv"
	"strings"

	"golang.org/x/xerrors"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
)

type stat struct {
	device string
	read   int64
	write  int64
}

func readStat(path string) (stats []stat, exists bool, err error) {
	exists, err = cgroups.ReadFile(path, func(file io.Reader) (exists bool, err error) {
		stats, exists, err = parseStat(file)
		return
	})
	return
}

func parseStat(reader io.Reader) ([]stat, bool, error) {
	parser := makeStatParser()

	if exists, err := cgroups.ParseFile(reader, parser.parse); err != nil || !exists {
		return nil, exists, err
	}

	stats, err := parser.consume()
	return stats, true, err
}

type statParser struct {
	stats    []stat
	current  stat
	gotTotal bool
}

func makeStatParser() statParser {
	p := statParser{}
	p.resetCurrent()
	return p
}

func (p *statParser) resetCurrent() {
	p.current = stat{
		read:  -1,
		write: -1,
	}
}

func (p *statParser) currentParsed() bool {
	return p.current.device != "" && p.current.read != -1 && p.current.write != -1
}

func (p *statParser) finalizeCurrent() bool {
	if !p.currentParsed() {
		return false
	}

	p.stats = append(p.stats, p.current)
	p.resetCurrent()

	return true
}

func (p *statParser) parse(line string) error {
	if !p.tryParse(line) {
		return xerrors.Errorf("Got an unexpected blkio stat line: %q", line)
	}
	return nil
}

func (p *statParser) tryParse(line string) bool {
	if p.gotTotal {
		return false
	}

	tokens := strings.Split(line, " ")
	if tokens[0] == "Total" {
		p.gotTotal = true
		return true
	} else if len(tokens) != 3 {
		return false
	}

	device, operation, countString := tokens[0], tokens[1], tokens[2]
	if p.current.device == "" {
		p.current.device = device
	} else if p.current.device != device {
		if !p.finalizeCurrent() {
			return false
		}
		p.current.device = device
	}

	var counter *int64
	switch operation {
	case "Read":
		counter = &p.current.read
	case "Write":
		counter = &p.current.write
	default:
		return true
	}
	if *counter != -1 {
		return false
	}

	count, err := strconv.ParseInt(countString, 10, 64)
	if err != nil || count < 0 {
		return false
	}

	*counter = count
	return true
}

func (p *statParser) consume() ([]stat, error) {
	if p.current.device != "" {
		if !p.finalizeCurrent() {
			return nil, xerrors.Errorf("Got an unexpected end of blkio stat file")
		}
	}
	return p.stats, nil
}
