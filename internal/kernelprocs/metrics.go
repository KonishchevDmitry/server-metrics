package kernelprocs

import (
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var cpuUsageMetric = metrics.MakeDescBuilder("kernel").Build(
	"cpu_usage", "CPU time consumed by kernel processes.", []string{"process"})
