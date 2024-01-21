package kernelprocs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tklauser/go-sysconf"
	"go.uber.org/zap"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
)

type Collector struct {
	logger         *zap.SugaredLogger
	clockFrequency float64

	lock          sync.Mutex
	kworkers      map[int]uint64
	kworkersUsage uint64
}

var _ prometheus.Collector = &Collector{}

func NewCollector(logger *zap.SugaredLogger) (*Collector, error) {
	clockFrequency, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil {
		return nil, fmt.Errorf("Failed to get SC_CLK_TCK value: %w", err)
	}

	return &Collector{
		logger:         logger,
		clockFrequency: float64(clockFrequency),
		kworkers:       make(map[int]uint64),
	}, nil
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- cpuUsageMetric
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	ctx := logging.WithLogger(context.Background(), c.logger)

	if err := c.observe(ctx, metrics); err != nil {
		logging.L(ctx).Errorf("Failed to collect kernel CPU usage: %s.", err)
	}
}

func (c *Collector) observe(ctx context.Context, metrics chan<- prometheus.Metric) error {
	const kworkersName = "kworkers"

	group := cgroups.NewGroup("/")

	pids, exists, err := group.PIDs()
	if err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("%q is not mounted", group.Path())
	}

	kworkers := make(map[int]uint64)
	names := map[string]struct{}{kworkersName: struct{}{}}

	logging.L(ctx).Debugf("Kernel CPU usage:")

	c.lock.Lock()
	defer c.lock.Unlock()

	kworkersUsage := c.kworkersUsage

	for _, pid := range pids {
		path := fmt.Sprintf("/proc/%d/stat", pid)

		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}

			if _, err := os.Stat("/proc/self/stat"); err != nil {
				if os.IsNotExist(err) {
					return errors.New("/proc is not mounted")
				}
				return err
			}

			continue
		}

		stat, ok := parseProcStat(data)
		if !ok {
			logging.L(ctx).Errorf("%q has an unexpected data: %q.", path, string(data))
			continue
		}

		// kworker processes constantly change their names displaying currently processing task, so we can't collect
		// they by name.
		if strings.HasPrefix(stat.name, "kworker/") {
			kworkers[pid] = stat.usage

			if prevUsage, ok := c.kworkers[pid]; ok {
				if stat.usage < prevUsage {
					logging.L(ctx).Errorf("Got CPU usage decrease for #%d kworker.", pid)
				} else {
					kworkersUsage += stat.usage - prevUsage
				}
			}

			continue
		} else if stat.usage == 0 {
			continue
		}

		if _, ok := names[stat.name]; ok {
			logging.L(ctx).Errorf("Got a duplicated kernel process name: %q.", stat.name)
			continue
		}
		names[stat.name] = struct{}{}

		usage := float64(stat.usage) / c.clockFrequency
		logging.L(ctx).Debugf("* #%d (%s): %v", pid, stat.name, usage)
		metrics <- prometheus.MustNewConstMetric(cpuUsageMetric, prometheus.CounterValue, usage, stat.name)
	}

	c.kworkers = kworkers
	c.kworkersUsage = kworkersUsage

	usage := float64(kworkersUsage) / c.clockFrequency
	logging.L(ctx).Debugf("* %s: %v", kworkersName, usage)
	metrics <- prometheus.MustNewConstMetric(cpuUsageMetric, prometheus.CounterValue, usage, kworkersName)

	return nil
}

type procStat struct {
	name  string
	usage uint64
}

func parseProcStat(data []byte) (procStat, bool) {
	nameStart := bytes.IndexByte(data, '(')
	nameEnd := bytes.LastIndexByte(data, ')')

	if nameStart == -1 || nameEnd < nameStart {
		return procStat{}, false
	}

	const statShift = 2
	statValues := bytes.Split(data[nameEnd:], []byte(" "))

	stimePos := 15 - statShift
	if len(statValues) <= stimePos {
		return procStat{}, false
	}

	stime, err := strconv.ParseUint(string(statValues[stimePos]), 10, 64)
	if err != nil {
		return procStat{}, false
	}

	return procStat{
		name:  string(data[nameStart+1 : nameEnd]),
		usage: stime,
	}, true
}
