package network

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

const subsystem = "network"
const familyLabel = "family"
const typeLabelName = "type"
const protocolLabelName = "protocol"

var metricBuilder = metrics.MakeDescBuilder(subsystem)

var inputRejectsIPsMetric = metricBuilder.Build(
	"input_rejects_ips", "Count of IP addresses with new input rejects.",
	[]string{familyLabel, typeLabelName})

func inputRejectsMetric() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "input_rejects_ports",
		Help:      "Count of ports with input rejects per IP.",
		Buckets:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 30},
	}, []string{familyLabel, typeLabelName, protocolLabelName})
}

var inputRejectsTOPIPMetric = metricBuilder.Build(
	"input_rejects_top_ip", "Count of ports with input rejects for the top IP.",
	[]string{familyLabel, typeLabelName, protocolLabelName})

var topForwardIPMetric = metricBuilder.Build(
	"forward_connections_top_ip", "Count of unique ports with new forward connections attempts for the top IP.",
	nil)

var inputRejectsSetSizeMetric = metricBuilder.Build(
	"port_connections_set_size", "Size of the sets storing unique ports with new input connections statistics.",
	[]string{familyLabel, protocolLabelName})

var forwardSetSizeMetric = metricBuilder.Build(
	"forward_connections_set_size", "Size of the sets storing unique ports with new forward connections statistics.",
	[]string{familyLabel})
