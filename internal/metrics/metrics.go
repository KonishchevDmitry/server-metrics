package metrics

import "github.com/prometheus/client_golang/prometheus"

const Namespace = "server"

var ErrorsMetric = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: Namespace,
	Subsystem: "metrics",
	Name:      "errors",
	Help:      "Metrics collection errors.",
})

func init() {
	prometheus.MustRegister(ErrorsMetric)
}

type DescBuilder struct {
	subsystem string
	labels    []string
}

func MakeDescBuilder(subsystem string) DescBuilder {
	return DescBuilder{subsystem: subsystem}
}

func (b DescBuilder) WithLabels(labels ...string) DescBuilder {
	b.labels = labels
	return b
}

func (b DescBuilder) Build(name string, help string, labels []string) *prometheus.Desc {
	name = prometheus.BuildFQName(Namespace, b.subsystem, name)
	labels = append(append([]string{}, b.labels...), labels...)
	return prometheus.NewDesc(name, help, labels, nil)
}
