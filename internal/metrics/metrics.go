package metrics

import "github.com/prometheus/client_golang/prometheus"

const Namespace = "server_services"
const ServiceLabel = "service"

func Labels(service string) prometheus.Labels {
	return prometheus.Labels{ServiceLabel: service}
}
