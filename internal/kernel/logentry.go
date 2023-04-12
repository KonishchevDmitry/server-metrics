package kernel

import (
	"bytes"
	"log/syslog"
	"strconv"
	"strings"
	"time"
)

type logEntry struct {
	priority syslog.Priority
	time     time.Duration
	message  string
}

func parseLogEntry(data []byte) (logEntry, bool) {
	delimiterPos := bytes.IndexByte(data, ';')
	if delimiterPos <= 0 {
		return logEntry{}, false
	}

	metadata := bytes.Split(data[:delimiterPos], []byte(","))
	if len(metadata) < 4 {
		return logEntry{}, false
	}

	priority, err := strconv.Atoi(string(metadata[0]))
	if err != nil || priority&(facilityMask|severityMask) != priority {
		return logEntry{}, false
	}

	timestamp, err := strconv.ParseInt(string(metadata[2]), 10, 64)
	if err != nil || timestamp < 0 {
		return logEntry{}, false
	}

	message := data[delimiterPos+1:]
	pos := bytes.IndexByte(message, '\n')
	if pos < 0 {
		return logEntry{}, false
	}
	message = message[:pos]

	return logEntry{
		priority: syslog.Priority(priority),
		time:     time.Duration(timestamp) * time.Microsecond,
		message:  strings.ReplaceAll(string(message), `\x0a`, "\n"),
	}, true
}

const facilityMask = 0xf8

func (e *logEntry) facility() syslog.Priority {
	return e.priority & facilityMask
}

const severityMask = 0x07

func (e *logEntry) severity() syslog.Priority {
	return e.priority & severityMask
}
