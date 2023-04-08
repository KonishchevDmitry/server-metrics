package cgroups

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

type Collector interface {
	Describe(descs chan<- *prometheus.Desc)
	Pre()
	Collect(ctx context.Context, service string, group *Group, exclude []string, metrics chan<- prometheus.Metric) (bool, error)
	Post(ctx context.Context)
}
