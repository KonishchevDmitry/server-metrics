package metrics

import "github.com/prometheus/client_golang/prometheus"

const Namespace = "server"

const BlockSubsystem = "block"
const MemorySubsystem = "memory"

const ServiceLabel = "service"

func Labels(serviceName string) prometheus.Labels {
	return prometheus.Labels{ServiceLabel: serviceName}
}
