package io

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var readsMetric = newMetric("reads", "Number of read operations issued to the disk by the service.")
var writesMetric = newMetric("writes", "Number of write operations issued to the disk by the service.")

var readBytesMetric = newMetric("read_bytes", "Number of bytes read from the disk by the service.")
var writtenBytesMetric = newMetric("written_bytes", "Number of bytes written to the disk by the service.")

const deviceLabel = "device"

func newMetric(name string, help string) *prometheus.GaugeVec {
	return metrics.ServiceMetric("blkio", name, help, deviceLabel)
}

func newLabels(service string, device string) prometheus.Labels {
	labels := metrics.ServiceLabels(service)
	labels[deviceLabel] = device
	return labels
}
