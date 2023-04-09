package cpu

import (
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var metricBuilder = metrics.MakeDescBuilder("services_cpu").WithLabels("service")

var userMetric = metricBuilder.Build("user", "CPU time consumed in user mode.", nil)
var systemMetric = metricBuilder.Build("system", "CPU time consumed in system (kernel) mode.", nil)
