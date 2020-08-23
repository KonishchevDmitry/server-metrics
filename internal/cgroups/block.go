package cgroups

import (
	"io"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
)

type blockStat struct {
	device string
	read   int64
	write  int64
}

func parseBlockStat(reader io.Reader) ([]blockStat, bool, error) {
	parser := makeBlockStatParser()

	if exists, err := parseFile(reader, parser.parse); err != nil || !exists {
		return nil, exists, err
	}

	stats, err := parser.consume()
	return stats, true, err
}

type blockStatParser struct {
	stats    []blockStat
	current  blockStat
	gotTotal bool
}

func makeBlockStatParser() blockStatParser {
	p := blockStatParser{}
	p.resetCurrent()
	return p
}

func (p *blockStatParser) resetCurrent() {
	p.current = blockStat{
		read:  -1,
		write: -1,
	}
}

func (p *blockStatParser) currentParsed() bool {
	return p.current.device != "" && p.current.read != -1 && p.current.write != -1
}

func (p *blockStatParser) finalizeCurrent() bool {
	if !p.currentParsed() {
		return false
	}

	p.stats = append(p.stats, p.current)
	p.resetCurrent()

	return true
}

func (p *blockStatParser) parse(line string) error {
	if !p.tryParse(line) {
		return xerrors.Errorf("Got an unexpected block stat line: %q", line)
	}
	return nil
}

func (p *blockStatParser) tryParse(line string) bool {
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

func (p *blockStatParser) consume() ([]blockStat, error) {
	if p.current.device != "" {
		if !p.finalizeCurrent() {
			return nil, xerrors.Errorf("Got an unexpected end of block stat file")
		}
	}
	return p.stats, nil
}
