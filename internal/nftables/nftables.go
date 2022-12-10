package nftables

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/samber/mo"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"

	"github.com/google/nftables"
)

// FIXME(konishchev): Alter it
const portScanThreshold = 10

var uniqueIPsMetric = metrics.NetworkMetric(
	"new_connections", "ips", "Count of IP addresses with new connection attempts.")

var topIPMetric = metrics.NetworkMetric(
	"port_connections", "top_ip", "Count of unique ports with new connections attempts for the top IP.")

const protocolLabelName = "protocol"

var setSizeMetric = metrics.NetworkMetric(
	"port_connections", "set_size", "Size of the sets storing unique ports with new connections statistics.",
	protocolLabelName)

type Collector struct {
	connection *nftables.Conn
	table      mo.Option[*nftables.Table]

	dryRun bool
	banned map[string]struct{}
}

func NewCollector(dryRun bool) (retCollector *Collector, retErr error) {
	connection, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return nil, fmt.Errorf("Unable to open netlink connection: %w", err)
	}
	return &Collector{
		connection: connection,
		dryRun:     dryRun,
	}, nil
}

func (c *Collector) Close(ctx context.Context) {
	if err := c.connection.CloseLasting(); err != nil {
		logging.L(ctx).Errorf("Failed to close netlink connection: %s.", err)
	}
}

func (c *Collector) Collect(ctx context.Context) {
	toBan, err := c.collect(ctx, c.banned)
	if err != nil {
		logging.L(ctx).Errorf("Failed to collect network metrics: %s.", err)
	}

	if !c.dryRun {
		// Give a time to fail2ban to react on the log message and remove the set elements on next iteration
		c.banned = toBan
	}
}

func (c *Collector) collect(ctx context.Context, banned map[string]struct{}) (map[string]struct{}, error) {
	addressFamilies := []*addressFamily{
		newAddressFamily(4, net.IPv4len, nftables.TypeIPAddr),
		newAddressFamily(6, net.IPv6len, nftables.TypeIP6Addr),
	}
	toDelete := make(map[*nftables.Set][]nftables.SetElement)

	table, err := c.getTable()
	if err != nil {
		return nil, err
	}

	for _, family := range addressFamilies {
		if size := family.dataType.Bytes; size != family.size {
			return nil, fmt.Errorf("Got an unexpected %s data type size: %d", family.name, size)
		}

		elementType, err := nftables.ConcatSetType(family.dataType, nftables.TypeInetService)
		if err != nil {
			return nil, err
		}

		for _, protocol := range []protocolFamily{
			makeProtocolFamily("TCP", func(stat *ipStat) { stat.tcp++ }),
			makeProtocolFamily("UDP", func(stat *ipStat) { stat.udp++ }),
		} {
			setName := fmt.Sprintf("%s%d_ports_connections", protocol.label, family.version)

			set, err := c.connection.GetSetByName(table, setName)
			if err != nil {
				return nil, fmt.Errorf("Failed to get %q set: %w", setName, err)
			}

			elements, err := c.connection.GetSetElements(set)
			if err != nil {
				return nil, fmt.Errorf("Failed to list %q set: %w", setName, err)
			}

			setSize := len(elements)
			logging.L(ctx).Debugf("%s %s ports connections set size: %d.", family.name, protocol.name, setSize)

			setLabels := metrics.NetworkLabels(family.label)
			setLabels[protocolLabelName] = protocol.label
			setSizeMetric.With(setLabels).Set(float64(setSize))

			for _, element := range elements {
				if size := len(element.Key); size != int(elementType.Bytes) {
					return nil, fmt.Errorf(
						"Got %q set element of an unexpected size: %d vs %d",
						setName, size, elementType.Bytes)
				}
				ip := net.IP(element.Key[:family.size]).String()

				if _, ok := banned[ip]; ok {
					toDelete[set] = append(toDelete[set], element)
					continue
				}

				stat, ok := family.stats[ip]
				if !ok {
					stat = &ipStat{}
					family.stats[ip] = stat
				}
				protocol.inc(stat)
			}
		}
	}

	if err := c.deleteBanned(ctx, toDelete); err != nil {
		return nil, err
	}

	toBan := make(map[string]struct{})

	for _, family := range addressFamilies {
		var topStat ipStat
		var topIP mo.Option[string]

		for ip, stat := range family.stats {
			if stat.tcp >= portScanThreshold || stat.udp >= portScanThreshold {
				logging.L(ctx).Warnf("Port scan detected: %s: %s.", ip, stat)
				toBan[ip] = struct{}{}
			} else if stat.total() > topStat.total() {
				topStat = *stat
				topIP = mo.Some(ip)
			}
		}

		uniqIPs := len(family.stats)
		logging.L(ctx).Debugf("Unique %s with new connection attempts: %d.", family.name, uniqIPs)
		uniqueIPsMetric.With(metrics.NetworkLabels(family.label)).Set(float64(uniqIPs))

		if topIP, ok := topIP.Get(); ok {
			logging.L(ctx).Debugf("Top %s with new connection attempts: %s: %s.", family.name, topIP, topStat)
		}
		topIPMetric.With(metrics.NetworkLabels(family.label)).Set(float64(topStat.total()))
	}

	return toBan, nil
}

func (c *Collector) getTable() (*nftables.Table, error) {
	if table, ok := c.table.Get(); ok {
		return table, nil
	}

	tables, err := c.connection.ListTablesOfFamily(nftables.TableFamilyINet)
	if err != nil {
		return nil, fmt.Errorf("Failed to list tables: %w", err)
	}

	const filterTableName = "filter"
	for _, table := range tables {
		if table.Name == filterTableName {
			c.table = mo.Some(table)
			return table, nil
		}
	}

	return nil, fmt.Errorf("Unable to find %q table", filterTableName)
}

func (c *Collector) deleteBanned(ctx context.Context, sets map[*nftables.Set][]nftables.SetElement) (retErr error) {
	var deletedElements int
	defer func() {
		if deletedElements == 0 {
			return
		}

		if err := c.connection.Flush(); err != nil {
			if retErr == nil {
				retErr = fmt.Errorf(
					"Unable to delete %d elements from ports connections tracking sets: %w",
					deletedElements, err)
			}
			return
		}

		logging.L(ctx).Infof("%d elements have been deleted from ports connections tracking sets.", deletedElements)
	}()

	for set, elements := range sets {
		if err := c.connection.SetDeleteElements(set, elements); err != nil {
			return fmt.Errorf("Unable to delete %d elements from %q set: %w", len(elements), set.Name, err)
		}
		deletedElements += len(elements)
	}

	return nil
}

type addressFamily struct {
	version int
	name    string
	label   string

	size     uint32
	dataType nftables.SetDatatype

	stats map[string]*ipStat
}

func newAddressFamily(version int, size uint32, dataType nftables.SetDatatype) *addressFamily {
	name := fmt.Sprintf("IPv%d", version)
	return &addressFamily{
		version: version,
		name:    name,
		label:   strings.ToLower(name),

		size:     size,
		dataType: dataType,

		stats: make(map[string]*ipStat),
	}
}

type protocolFamily struct {
	name  string
	label string
	inc   func(stat *ipStat)
}

func makeProtocolFamily(name string, inc func(stat *ipStat)) protocolFamily {
	return protocolFamily{
		name:  name,
		label: strings.ToLower(name),
		inc:   inc,
	}
}

type ipStat struct {
	tcp int
	udp int
}

func (s *ipStat) total() int {
	return s.tcp + s.udp
}

func (s ipStat) String() string {
	return fmt.Sprintf("%d TCP, %d UDP", s.tcp, s.udp)
}
