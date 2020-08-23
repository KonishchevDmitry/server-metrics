package blkio

import (
	"context"
	"path"

	"github.com/c2h5oh/datasize"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

const controller = "blkio"
const deviceLabel = "device"

var readsMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: controller,
		Name:      "reads",
		Help:      "Number of read operations issued to the disk by the service.",
	},
	[]string{metrics.ServiceLabel, deviceLabel},
)

var writesMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: controller,
		Name:      "writes",
		Help:      "Number of write operations issued to the disk by the service.",
	},
	[]string{metrics.ServiceLabel, deviceLabel},
)

var readBytesMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: controller,
		Name:      "read_bytes",
		Help:      "Number of bytes read from the disk by the service.",
	},
	[]string{metrics.ServiceLabel, deviceLabel},
)

var writtenBytesMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: controller,
		Name:      "written_bytes",
		Help:      "Number of bytes written to the disk by the service.",
	},
	[]string{metrics.ServiceLabel, deviceLabel},
)

func init() {
	prometheus.MustRegister(readsMetric)
	prometheus.MustRegister(writesMetric)

	prometheus.MustRegister(readBytesMetric)
	prometheus.MustRegister(writtenBytesMetric)
}

type Collector struct {
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{}
}

func (o *Collector) Controller() string {
	return controller
}

func (o *Collector) Collect(ctx context.Context, slice *cgroups.Slice) (bool, error) {
	var statSuffix string
	if slice.Total {
		statSuffix = "_recursive"
	}

	opsStats, exists, err := readStat(path.Join(slice.Path, "blkio.throttle.io_serviced"+statSuffix))
	if !exists || err != nil {
		return exists, err
	}

	bytesStats, exists, err := readStat(path.Join(slice.Path, "blkio.throttle.io_service_bytes"+statSuffix))
	if !exists || err != nil {
		return exists, err
	}

	for _, stat := range opsStats {
		device := stat.device
		logging.L(ctx).Debugf("* %s:%s: reads=%d, writes=%d", slice.Service, device, stat.read, stat.write)
		if false {
			readsMetric.With(labels(slice.Service, device)).Set(float64(stat.read))
		}
	}

	for _, stat := range bytesStats {
		device := stat.device
		logging.L(ctx).Debugf(
			"* %s:%s: read=%s, written=%s", slice.Service, device,
			datasize.ByteSize(stat.read), datasize.ByteSize(stat.write))
	}

	return true, nil
}

func labels(service string, device string) prometheus.Labels {
	labels := metrics.Labels(service)
	labels[deviceLabel] = device
	return labels
}
