package meminfo

import (
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

var meminfoMetric = metrics.MakeDescBuilder("memory").Build(
	"meminfo", "/proc/meminfo counters.", []string{"name"})
