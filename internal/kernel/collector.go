package kernel

import (
	"context"
	"errors"
	"fmt"
	"log/syslog"
	"os"
	"syscall"
	"time"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

type Collector struct {
	kmsg         *os.File
	newMessages  chan logEntry
	messageGroup []logEntry
	classifier   *classifier
	waitGroup    util.WaitGroup
	errors       *prometheus.CounterVec
}

var _ prometheus.Collector = &Collector{}

func NewCollector(ctx context.Context) (*Collector, error) {
	path := "/dev/kmsg"

	kmsg, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s: %w", path, err)
	}

	c := &Collector{
		kmsg:        kmsg,
		newMessages: make(chan logEntry),
		classifier:  newClassifier(false),
		errors:      newErrorsMetric(),
	}
	c.waitGroup.Run(func() {
		defer close(c.newMessages)
		if err := c.logReader(ctx); err != nil {
			logging.L(ctx).Errorf("Kernel collector has crashed: %s.", err)
		}
	})
	c.waitGroup.Run(func() {
		c.logProcessor(ctx)
	})

	return c, nil
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	c.errors.Describe(descs)
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	c.errors.Collect(metrics)
}

func (c *Collector) logReader(ctx context.Context) error {
	buf := make([]byte, 1024)

	for {
		size, err := c.kmsg.Read(buf)
		if err != nil {
			switch {
			case errors.Is(err, os.ErrClosed):
				return nil

			case errors.Is(err, syscall.EINVAL):
				logging.L(ctx).Errorf("Got a too big message from kernel log. The message is dropped.")
				continue

			case errors.Is(err, syscall.EPIPE):
				logging.L(ctx).Errorf(
					"Error while reading kernel log: some messages have been overwritten before we've read them.")
				continue

			default:
				return fmt.Errorf("Error while reading kernel log: %w", err)
			}
		}

		data := buf[:size]

		entry, ok := parseLogEntry(data)
		if !ok {
			logging.L(ctx).Errorf("Got an invalid data from kernel log: %q.", data)
			continue
		} else if entry.facility() != syslog.LOG_KERN || entry.severity() > syslog.LOG_ERR {
			continue
		}

		c.newMessages <- entry
	}
}

func (c *Collector) logProcessor(ctx context.Context) {
	infiniteChan := make(<-chan time.Time)
	timerChan := infiniteChan

	var timer *time.Timer
	startTimer := func(duration time.Duration) {
		timer = time.NewTimer(duration)
		timerChan = timer.C
	}
	stopTimer := func() {
		if timer != nil {
			timer.Stop()
			timer = nil
			timerChan = infiniteChan
		}
	}

	defer stopTimer()
	defer c.processGroup(ctx)

	const maxGroupSize = 100
	const maxGroupTime = 10 * time.Millisecond

	for {
		select {
		case entry, ok := <-c.newMessages:
			if !ok {
				return
			}

			if groupTime, ok := c.groupTime(); ok && entry.time-groupTime > maxGroupTime {
				c.processGroup(ctx)
				stopTimer()
			}

			c.messageGroup = append(c.messageGroup, entry)

			if len(c.messageGroup) >= maxGroupSize {
				c.processGroup(ctx)
				stopTimer()
			} else if timer == nil {
				startTimer(maxGroupTime)
			}

		case <-timerChan:
			stopTimer()

			if groupTime, ok := c.groupTime(); ok {
				timeLeft := groupTime + maxGroupTime - util.Uptime()

				if timeLeft <= 0 {
					c.processGroup(ctx)
				} else {
					startTimer(timeLeft)
				}
			}
		}
	}
}

func (c *Collector) groupTime() (time.Duration, bool) {
	if len(c.messageGroup) == 0 {
		return 0, false
	}
	return c.messageGroup[0].time, true
}

func (c *Collector) processGroup(ctx context.Context) {
	if len(c.messageGroup) == 0 {
		return
	}

	var messages []string
	for _, entry := range c.messageGroup {
		messages = append(messages, entry.message)
	}
	c.messageGroup = c.messageGroup[:0]

	for _, errorType := range c.classifier.classify(ctx, messages) {
		c.errors.WithLabelValues(string(errorType)).Inc()
	}
}

func (c *Collector) Close(ctx context.Context) {
	if err := c.kmsg.Close(); err != nil && !errors.Is(err, syscall.EINTR) {
		logging.L(ctx).Errorf("Failed to close kernel log file: %s.", err)
	}
	c.waitGroup.Wait()
}
