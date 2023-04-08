package io

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var readsMetric = newDesc("reads", "Number of read operations issued to the disk by the service.")
var writesMetric = newDesc("writes", "Number of write operations issued to the disk by the service.")

var readBytesMetric = newDesc("read_bytes", "Number of bytes read from the disk by the service.")
var writtenBytesMetric = newDesc("written_bytes", "Number of bytes written to the disk by the service.")

func newDesc(name string, help string) *prometheus.Desc {
	return metrics.ServiceDesc("blkio", name, help, "device")
}
