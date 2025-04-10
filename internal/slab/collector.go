package slab

import (
	"context"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type Collector struct {
	logger  *zap.SugaredLogger
	details *slabDetailsCache
}

var _ prometheus.Collector = &Collector{}

func NewCollector(logger *zap.SugaredLogger) *Collector {
	return &Collector{
		logger:  logger,
		details: newSlabDetailsCache(),
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- sizeMetric
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	ctx := logging.WithLogger(context.Background(), c.logger)

	if err := c.observe(ctx, metrics); err != nil {
		logging.L(ctx).Errorf("Failed to collect slab metrics: %s.", err)
	}
}

func (c *Collector) observe(ctx context.Context, metrics chan<- prometheus.Metric) error {
	type fullSlabInfo struct {
		slabInfo
		reclaimable bool
	}

	infos, err := ReadSlabInfo()
	if err != nil {
		return err
	}

	var (
		slabs             = make([]fullSlabInfo, 0, len(infos))
		reclaimableSize   int64
		unreclaimableSize int64
	)

	for _, info := range infos {
		reclaimable, err := c.details.isReclaimable(info.name)
		if err != nil {
			return err
		}

		if reclaimable {
			reclaimableSize += info.size
		} else {
			unreclaimableSize += info.size
		}

		slabs = append(slabs, fullSlabInfo{
			slabInfo:    info,
			reclaimable: reclaimable,
		})
	}

	logging.L(ctx).Debugf("slab: %d reclaimable, %d unreclaimable", reclaimableSize, unreclaimableSize)

	for _, slab := range slabs {
		typeLabel := "unreclaimable"
		if slab.reclaimable {
			typeLabel = "reclaimable"
		}
		metrics <- prometheus.MustNewConstMetric(sizeMetric, prometheus.GaugeValue, float64(slab.size), slab.name, typeLabel)
	}

	return nil
}
