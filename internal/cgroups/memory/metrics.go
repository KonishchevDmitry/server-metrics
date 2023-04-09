package memory

import (
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var metricBuilder = metrics.MakeDescBuilder("services_memory").WithLabels("service")

var rssMetric = metricBuilder.Build("rss", "Anonymous and swap cache memory usage.", nil)
var swapMetric = metricBuilder.Build("swap", "Non-cached swap usage.", nil)
var cacheMetric = metricBuilder.Build("cache", "Page cache memory usage.", nil)
var kernelMetric = metricBuilder.Build("kernel", "Kernel data structures.", nil)
