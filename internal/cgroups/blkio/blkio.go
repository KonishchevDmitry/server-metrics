package blkio

import (
	"context"
	"os"
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
		Namespace: metrics.ServicesNamespace,
		Subsystem: controller,
		Name:      "reads",
		Help:      "Number of read operations issued to the disk by the service.",
	},
	[]string{metrics.ServiceLabel, deviceLabel},
)

var writesMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.ServicesNamespace,
		Subsystem: controller,
		Name:      "writes",
		Help:      "Number of write operations issued to the disk by the service.",
	},
	[]string{metrics.ServiceLabel, deviceLabel},
)

var readBytesMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.ServicesNamespace,
		Subsystem: controller,
		Name:      "read_bytes",
		Help:      "Number of bytes read from the disk by the service.",
	},
	[]string{metrics.ServiceLabel, deviceLabel},
)

var writtenBytesMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metrics.ServicesNamespace,
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
	devices map[string]string
}

var _ cgroups.Collector = &Collector{}

func NewCollector() *Collector {
	return &Collector{make(map[string]string)}
}

func (c *Collector) Controller() string {
	return controller
}

func (c *Collector) Collect(ctx context.Context, slice *cgroups.Slice) (bool, error) {
	opsStats, exists, err := readStat(slice, false, slice.Total)
	if !exists || err != nil {
		return exists, err
	}

	bytesStats, exists, err := readStat(slice, true, slice.Total)
	if !exists || err != nil {
		return exists, err
	}

	for _, stat := range opsStats {
		device := c.getDeviceName(ctx, stat.device)
		logging.L(ctx).Debugf("* %s:%s: reads=%d, writes=%d", slice.Service, device, stat.read, stat.write)
		readsMetric.With(labels(slice.Service, device)).Set(float64(stat.read))
		writesMetric.With(labels(slice.Service, device)).Set(float64(stat.write))
	}

	for _, stat := range bytesStats {
		device := c.getDeviceName(ctx, stat.device)
		logging.L(ctx).Debugf(
			"* %s:%s: read=%s, written=%s", slice.Service, device,
			datasize.ByteSize(stat.read), datasize.ByteSize(stat.write))
		readBytesMetric.With(labels(slice.Service, device)).Set(float64(stat.read))
		writtenBytesMetric.With(labels(slice.Service, device)).Set(float64(stat.write))
	}

	return true, nil
}

func (c *Collector) getDeviceName(ctx context.Context, device string) string {
	if name, ok := c.devices[device]; ok {
		return name
	}

	name, err := readDeviceLink(device)
	if err != nil {
		logging.L(ctx).Errorf("Failed to resolve %q device: %s.", device, err)
		name = device
	}

	name = path.Base(name)
	c.devices[device] = name
	return name
}

func readDeviceLink(device string) (string, error) {
	name, err := os.Readlink(path.Join("/dev/char", device))
	if err != nil && os.IsNotExist(err) {
		name, err = os.Readlink(path.Join("/dev/block", device))
	}
	return name, err
}

func labels(service string, device string) prometheus.Labels {
	labels := metrics.Labels(service)
	labels[deviceLabel] = device
	return labels
}
