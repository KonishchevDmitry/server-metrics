package zswap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

type Collector struct {
	logger   *zap.SugaredLogger
	pageSize int
}

var _ prometheus.Collector = &Collector{}

func NewCollector(logger *zap.SugaredLogger) *Collector {
	return &Collector{
		logger:   logger,
		pageSize: os.Getpagesize(),
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- enabledMetric
	descs <- maxPoolPercentMetric
	descs <- poolSizeMetric
	descs <- storedSizeMetric
	descs <- compressionRatioMetric
	descs <- rejectsMetric
	descs <- errorsMetric
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	ctx := logging.WithLogger(context.Background(), c.logger)

	if err := c.observe(ctx, metrics); err != nil {
		logging.L(ctx).Errorf("Failed to collect zswap metrics: %s.", err)
	}
}

func (c *Collector) observe(ctx context.Context, metrics chan<- prometheus.Metric) error {
	parametersPath := "/sys/module/zswap/parameters"
	statisticsPath := "/sys/kernel/debug/zswap"

	enabledPath := path.Join(parametersPath, "enabled")
	enabledString, err := readString(enabledPath)
	if err != nil {
		return err
	}

	var enabledValue float64
	switch enabledString {
	case "Y":
		enabledValue = 1
	case "N":
	default:
		return fmt.Errorf("got an unexpected %q value: %q", enabledPath, enabledString)
	}

	zpool, err := readString(path.Join(parametersPath, "zpool"))
	if err != nil {
		return err
	}

	compressor, err := readString(path.Join(parametersPath, "compressor"))
	if err != nil {
		return err
	}

	maxPoolPercent, err := readValue(path.Join(parametersPath, "max_pool_percent"))
	if err != nil {
		return err
	}

	poolSize, err := readValue(path.Join(statisticsPath, "pool_total_size"))
	if err != nil {
		return err
	}

	storedPages, err := readValue(path.Join(statisticsPath, "stored_pages"))
	if err != nil {
		return err
	}
	storedSize := storedPages * float64(c.pageSize)

	poolLimitHits, err := readValue(path.Join(statisticsPath, "pool_limit_hit"))
	if err != nil {
		return err
	}

	allocationFailureRejects, err := readValue(path.Join(statisticsPath, "reject_alloc_fail"))
	if err != nil {
		return err
	}

	kmemcacheRejects, err := readValue(path.Join(statisticsPath, "reject_kmemcache_fail"))
	if err != nil {
		return err
	}
	allocationFailureRejects += kmemcacheRejects

	poorCompressionRejects, err := readValue(path.Join(statisticsPath, "reject_compress_poor"))
	if err != nil {
		return err
	}

	reclaimErrors, err := readValue(path.Join(statisticsPath, "reject_reclaim_fail"))
	if err != nil {
		return err
	}

	compressionErrors, err := readValue(path.Join(statisticsPath, "reject_compress_fail"))
	if err != nil {
		return err
	}

	logging.L(ctx).Debugf("zswap:\n"+
		"* status: enabled=%v, zpool=%s, compressor=%s, max_pool_percent=%.0f, pool_size=%0.f, stored_size=%0.f\n"+
		"* rejects: %s=%0.f, %s=%0.f, %s=%0.f\n"+
		"* errors: %s=%0.f, %s=%0.f",
		enabledValue == 1, zpool, compressor, maxPoolPercent, poolSize, storedSize,
		poolLimitReachedReject, poolLimitHits, allocationFailureReject, allocationFailureRejects, poorCompressionReject, poorCompressionRejects,
		reclaimError, reclaimErrors, compressionError, compressionErrors)

	metrics <- prometheus.MustNewConstMetric(enabledMetric, prometheus.GaugeValue, enabledValue, zpool, compressor)
	metrics <- prometheus.MustNewConstMetric(maxPoolPercentMetric, prometheus.GaugeValue, maxPoolPercent)
	metrics <- prometheus.MustNewConstMetric(poolSizeMetric, prometheus.GaugeValue, poolSize)
	metrics <- prometheus.MustNewConstMetric(storedSizeMetric, prometheus.GaugeValue, storedSize)

	if poolSize != 0 {
		metrics <- prometheus.MustNewConstMetric(compressionRatioMetric, prometheus.GaugeValue, storedSize/poolSize)
	}

	metrics <- prometheus.MustNewConstMetric(rejectsMetric, prometheus.CounterValue, poolLimitHits, poolLimitReachedReject)
	metrics <- prometheus.MustNewConstMetric(rejectsMetric, prometheus.CounterValue, allocationFailureRejects, allocationFailureReject)
	metrics <- prometheus.MustNewConstMetric(rejectsMetric, prometheus.CounterValue, poorCompressionRejects, poorCompressionReject)

	metrics <- prometheus.MustNewConstMetric(errorsMetric, prometheus.CounterValue, reclaimErrors, reclaimError)
	metrics <- prometheus.MustNewConstMetric(errorsMetric, prometheus.CounterValue, compressionErrors, compressionError)

	return nil
}

func readString(path string) (string, error) {
	var value string

	err := util.ReadFile(path, func(file io.Reader) error {
		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		value = string(bytes.TrimRight(data, "\n"))
		return nil
	})

	return value, err
}

func readValue(path string) (float64, error) {
	var value float64

	err := util.ReadFile(path, func(file io.Reader) error {
		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		stringValue := string(bytes.TrimRight(data, "\n"))

		integer, err := strconv.ParseUint(stringValue, 10, 64)
		if err != nil {
			return err
		}

		value = float64(integer)
		return nil
	})

	return value, err
}
