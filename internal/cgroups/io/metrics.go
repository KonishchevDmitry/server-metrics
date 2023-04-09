package io

import (
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var metricBuilder = metrics.MakeDescBuilder("services_blkio").WithLabels("service", "device")

var readsMetric = metricBuilder.Build("reads", "Number of read operations issued to the disk by the service.", nil)
var writesMetric = metricBuilder.Build("writes", "Number of write operations issued to the disk by the service.", nil)
var readBytesMetric = metricBuilder.Build("read_bytes", "Number of bytes read from the disk by the service.", nil)
var writtenBytesMetric = metricBuilder.Build("written_bytes", "Number of bytes written to the disk by the service.", nil)
