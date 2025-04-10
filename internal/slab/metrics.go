package slab

import (
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var metricBuilder = metrics.MakeDescBuilder("kernel_slab")

var sizeMetric = metricBuilder.Build(
	"size", "Current slab size.", []string{"name", "type"})
