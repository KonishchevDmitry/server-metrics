package io

import (
	"github.com/prometheus/client_golang/prometheus"
)

var readsMetric = prometheus.NewDesc(
	"blkio_reads", "Number of read operations issued to the disk by the service.",
	metricLabels, nil)

var writesMetric = prometheus.NewDesc(
	"blkio_writes", "Number of write operations issued to the disk by the service.",
	metricLabels, nil)

var readBytesMetric = prometheus.NewDesc(
	"blkio_read_bytes", "Number of bytes read from the disk by the service.",
	metricLabels, nil)

var writtenBytesMetric = prometheus.NewDesc(
	"blkio_written_bytes", "Number of bytes written to the disk by the service.",
	metricLabels, nil)

var metricLabels = []string{"service", "device"}
