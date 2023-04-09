package network

import (
	"github.com/prometheus/client_golang/prometheus"
)

const familyLabel = "family"
const typeLabelName = "type"
const protocolLabelName = "protocol"

var uniqueInputIPsMetric = prometheus.NewDesc(
	"new_connections_ips", "Count of IP addresses with new input connection attempts.",
	[]string{familyLabel, typeLabelName}, nil)

func inputConnectionsMetric() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "new_connections_ports",
		Help:    "Count of unique ports with new input connections attempts per IP.",
		Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 30},
	}, []string{familyLabel, typeLabelName, protocolLabelName})
}

var topInputIPMetric = prometheus.NewDesc(
	"port_connections_top_ip", "Count of unique ports with new input connections attempts for the top IP.",
	[]string{familyLabel, typeLabelName, protocolLabelName}, nil)

var topForwardIPMetric = prometheus.NewDesc(
	"forward_connections_top_ip", "Count of unique ports with new forward connections attempts for the top IP.",
	nil, nil)

var inputSetSizeMetric = prometheus.NewDesc(
	"port_connections_set_size", "Size of the sets storing unique ports with new input connections statistics.",
	[]string{familyLabel, protocolLabelName}, nil)

var forwardSetSizeMetric = prometheus.NewDesc(
	"forward_connections_set_size", "Size of the sets storing unique ports with new forward connections statistics.",
	[]string{familyLabel}, nil)
