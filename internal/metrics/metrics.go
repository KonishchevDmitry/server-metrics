package metrics

import "github.com/prometheus/client_golang/prometheus"

const Namespace = "server"
const FamilyLabel = "family"
const serviceLabel = "service"

type GenericMetric interface {
	Reset()
}

func Metric(namespace string, subsystem string, name string, help string, labels ...string) *prometheus.GaugeVec {
	metric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace + "_" + namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}, labels)
	prometheus.MustRegister(metric)
	return metric
}

func Histogram(namespace string, subsystem string, name string, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	metric := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace + "_" + namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labels)
	prometheus.MustRegister(metric)
	return metric
}

func NetworkMetric(subsystem string, name string, help string, labels ...string) *prometheus.GaugeVec {
	return Metric("network", subsystem, name, help, labels...)
}

func NetworkHistogram(subsystem string, name string, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	return Histogram("network", subsystem, name, help, buckets, labels...)
}

func NetworkLabels(family string) prometheus.Labels {
	return prometheus.Labels{FamilyLabel: family}
}

func ServiceMetric(subsystem string, name string, help string, labels ...string) *prometheus.GaugeVec {
	labels = append([]string{serviceLabel}, labels...)
	return Metric("services", subsystem, name, help, labels...)
}

func ServiceLabels(service string) prometheus.Labels {
	return prometheus.Labels{serviceLabel: service}
}
