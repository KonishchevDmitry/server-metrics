package zswap

import (
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var metricBuilder = metrics.MakeDescBuilder("kernel_zswap")

var enabledMetric = metricBuilder.Build(
	"enabled", "Current zswap status.", []string{"compressor"})

var maxPoolPercentMetric = metricBuilder.Build(
	"max_pool_percent", "The maximum percentage of memory that the compressed pool can occupy.", nil)

var poolSizeMetric = metricBuilder.Build(
	"pool_size", "Current pool size.", nil)

var storedSizeMetric = metricBuilder.Build(
	"stored_size", "Amount of data stored in the pool.", nil)

var compressionRatioMetric = metricBuilder.Build(
	"compression_ratio", "Current compression ratio.", nil)

var rejectsMetric = metricBuilder.Build(
	"rejects_total", "The reason why page hasn't been saved to cache and was saved to swap (doesn't cover all cases such as accept_threshold_percent and shrinker_enabled).",
	[]string{"type"})

const (
	poolLimitReachedReject  = "pool-limit-reached" // Pool limit has been reached
	allocationFailureReject = "allocation-failure" // Unable to allocate memory for the cache entry
	poorCompressionReject   = "poor-compression"   // Compressed page was to big to use it in cache
)

var errorsMetric = metricBuilder.Build(
	"errors_total", "Unexpected errors.", []string{"type"})

const (
	reclaimError     = "reclaim"     // Failed to reclaim (move to swap) a compressed page
	compressionError = "compression" // Compression algorithm error
)
