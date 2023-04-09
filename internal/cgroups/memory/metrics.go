package memory

import "github.com/prometheus/client_golang/prometheus"

var rssMetric = prometheus.NewDesc(
	"memory_rss", "Anonymous and swap cache memory usage.",
	metricLabels, nil)

var swapMetric = prometheus.NewDesc(
	"memory_swap", "Non-cached swap usage.",
	metricLabels, nil)

var cacheMetric = prometheus.NewDesc(
	"memory_cache", "Page cache memory usage.",
	metricLabels, nil)

var kernelMetric = prometheus.NewDesc(
	"memory_kernel", "Kernel data structures.",
	metricLabels, nil)

var metricLabels = []string{"service"}
