package collector

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/classifier"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/cpu"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/io"
	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/memory"
)

type Collector struct {
	logger     *zap.SugaredLogger
	classifier *classifier.Classifier

	lock       sync.Mutex
	races      *cgroups.RaceController
	collectors []cgroups.Collector
}

var _ prometheus.Collector = &Collector{}

func NewCollector(logger *zap.SugaredLogger, classifier *classifier.Classifier, races *cgroups.RaceController) *Collector {
	return &Collector{
		logger:     logger,
		classifier: classifier,
		races:      races,
		collectors: []cgroups.Collector{
			cpu.NewCollector(races),
			memory.NewCollector(races),
			io.NewCollector(races),
		},
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	for _, collector := range c.collectors {
		collector.Describe(descs)
	}
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	ctx := logging.WithLogger(context.Background(), c.logger)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.races.OnCollectionStarted()

	for _, collector := range c.collectors {
		collector.Pre()
	}

	root := cgroups.NewGroup("/", c.races)
	services := make(map[string]string)

	exists, err := c.observe(ctx, root, services, metrics)
	if err == nil && !exists {
		err = fmt.Errorf("%q is not mounted", root.Path())
	}
	if err != nil {
		logging.L(ctx).Errorf("Failed to observe cgroups hierarchy: %s.", err)
		return
	}

	for _, collector := range c.collectors {
		collector.Post(ctx)
	}

	c.races.OnCollectionFinished()
}

func (c *Collector) observe(
	ctx context.Context, group *cgroups.Group, services map[string]string, metrics chan<- prometheus.Metric,
) (bool, error) {
	classification, classified, err := c.classifier.ClassifySlice(ctx, group.Name)
	if err != nil {
		logging.L(ctx).Errorf("Failed to classify %q cgroup: %s.", group.Name, err)
		return true, nil
	}

	var needsCollection bool

	if totalExcluding, ok := classification.TotalExcluding.Get(); ok {
		for _, name := range totalExcluding {
			if _, err := c.observe(ctx, group.Child(name), services, metrics); err != nil {
				return false, err
			}
		}
		needsCollection = true
	} else {
		var observeChildren bool

		if group.IsRoot() {
			needsCollection = true
			observeChildren = true
		} else {
			hasProcesses, exists, err := group.HasProcesses()
			if err != nil || !exists {
				return exists, err
			}
			needsCollection = hasProcesses
			observeChildren = !hasProcesses
		}

		if observeChildren {
			children, exists, err := group.Children()
			if err != nil || !exists {
				return exists, err
			}

			for _, child := range children {
				if exists, err := c.observe(ctx, child, services, metrics); err != nil {
					return false, err
				} else if !exists {
					logging.L(ctx).Debugf("%q has been deleted during discovering.", child.Path())
				}
			}
		}
	}

	if !needsCollection {
		return true, nil
	} else if !classified {
		logging.L(ctx).Errorf("Unable to classify %q cgroup.", group.Name)
		return true, nil
	}

	if otherGroup, ok := services[classification.Service]; ok {
		logging.L(ctx).Errorf("Both %q and %q resolve to %q service.", otherGroup, group.Name, classification.Service)
		return true, nil
	}
	services[classification.Service] = group.Name

	if exists, err := c.collect(ctx, classification.Service, group, classification.TotalExcluding.OrEmpty(), metrics); err != nil {
		logging.L(ctx).Errorf("Failed to collect metrics for %s cgroup: %s.", group.Name, err)
	} else if !exists {
		return false, nil
	}

	return true, nil
}

func (c *Collector) collect(
	ctx context.Context, service string, group *cgroups.Group, exclude []string, metrics chan<- prometheus.Metric,
) (bool, error) {
	if logger := logging.L(ctx); logger.Desugar().Core().Enabled(zap.DebugLevel) {
		var buf bytes.Buffer
		_, _ = fmt.Fprintf(&buf, "Collecting %s", group.Name)

		if len(exclude) != 0 {
			buf.WriteString(" (excluding ")
			for index, name := range exclude {
				if index != 0 {
					buf.WriteByte(',')
				}
				buf.WriteString(name)
			}
			buf.WriteByte(')')
		}

		_, _ = fmt.Fprintf(&buf, " as %s:", service)
		logger.Debug(buf.String())
	}

	for _, collector := range c.collectors {
		if exists, err := collector.Collect(ctx, service, group, exclude, metrics); err != nil || !exists {
			return exists, err
		}
	}

	return true, nil
}
