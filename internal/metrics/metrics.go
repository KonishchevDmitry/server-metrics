package metrics

import "github.com/prometheus/client_golang/prometheus"

const Namespace = "server"
const serviceLabel = "service"

func ServiceMetric(subsystem string, name string, help string, labels ...string) *prometheus.GaugeVec {
	metric := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace + "_services",
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		},
		append([]string{serviceLabel}, labels...),
	)
	prometheus.MustRegister(metric)
	return metric
}

func ServiceLabels(service string) prometheus.Labels {
	return prometheus.Labels{serviceLabel: service}
}
