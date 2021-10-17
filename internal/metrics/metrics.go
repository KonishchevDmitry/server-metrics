package metrics

import "github.com/prometheus/client_golang/prometheus"

const Namespace = "server"
const ServicesNamespace = Namespace + "_services"
const ServiceLabel = "service"

func NewServiceMetric(subsystem string, name string, help string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: ServicesNamespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		},
		[]string{ServiceLabel},
	)
}

func Labels(service string) prometheus.Labels {
	return prometheus.Labels{ServiceLabel: service}
}
