package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"

	"github.com/google/nftables"
	"github.com/samber/mo"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

const typeLabelName = "type"
const protocolLabelName = "protocol"

var uniqueInputIPsMetric = metrics.NetworkMetric(
	"new_connections", "ips", "Count of IP addresses with new input connection attempts.",
	metrics.FamilyLabel, typeLabelName)

var inputConnectionsMetric = metrics.NetworkHistogram(
	"new_connections", "ports", "Count of unique ports with new input connections attempts per IP.",
	[]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 30},
	metrics.FamilyLabel, typeLabelName, protocolLabelName)

var topInputIPMetric = metrics.NetworkMetric(
	"port_connections", "top_ip", "Count of unique ports with new input connections attempts for the top IP.",
	metrics.FamilyLabel, typeLabelName, protocolLabelName)

var topForwardIPMetric = metrics.NetworkMetric(
	"forward_connections", "top_ip", "Count of unique ports with new forward connections attempts for the top IP.")

var inputSetSizeMetric = metrics.NetworkMetric(
	"port_connections", "set_size", "Size of the sets storing unique ports with new input connections statistics.",
	metrics.FamilyLabel, protocolLabelName)

var forwardSetSizeMetric = metrics.NetworkMetric(
	"forward_connections", "set_size", "Size of the sets storing unique ports with new forward connections statistics.",
	metrics.FamilyLabel)

var allMetrics = []metrics.GenericMetric{
	uniqueInputIPsMetric, inputConnectionsMetric,
	topInputIPMetric, topForwardIPMetric,
	inputSetSizeMetric, forwardSetSizeMetric,
}

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
	for _, metric := range allMetrics {
		metric.Reset()
	}

	toBan, err := c.collectInputIPs(ctx, c.banned)
	if err == nil {
		err = c.collectForwardIPs(ctx)
	}
	if err != nil {
		logging.L(ctx).Errorf("Failed to collect network metrics: %s.", err)
	}

	if !c.dryRun {
		// Give a time to fail2ban to react on the log message and remove the set elements on next iteration
		c.banned = toBan
	}
}

func (c *Collector) collectInputIPs(ctx context.Context, banned map[string]struct{}) (map[string]struct{}, error) {
	logging.L(ctx).Debugf("Collecting input IPs:")

	addressFamilies := getAddressFamilies()
	protocols := getProtocolFamilies()

	toDelete := make(map[*nftables.Set][]nftables.SetElement)

	portType := nftables.TypeInetService
	if size := portType.Bytes; size != 2 {
		return nil, fmt.Errorf("Got an unexpected port data type size: %d", size)
	}
	portTypePadding := 4 - portType.Bytes

	localNetworks, err := getLocalNetworks()
	if err != nil {
		return nil, err
	}

	for _, family := range addressFamilies {
		if size := family.dataType.Bytes; size != family.size {
			return nil, fmt.Errorf("Got an unexpected %s data type size: %d", family.name, size)
		}

		elementType, err := nftables.ConcatSetType(family.dataType, portType)
		if err != nil {
			return nil, err
		} else if size := elementType.Bytes; size != family.dataType.Bytes+portType.Bytes+portTypePadding {
			return nil, fmt.Errorf("Got an unexpected %s data type size: %d", elementType.Name, size)
		}

		for _, protocol := range protocols {
			setName := fmt.Sprintf("%s%d_ports_connections", protocol.label, family.version)

			set, elements, err := c.getSet(setName)
			if err != nil {
				return nil, err
			}

			setSize := len(elements)
			logging.L(ctx).Debugf("* %s %s ports connections set size: %d.", family.name, protocol.name, setSize)

			setLabels := metrics.NetworkLabels(family.label)
			setLabels[protocolLabelName] = protocol.label
			inputSetSizeMetric.With(setLabels).Set(float64(setSize))

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

				port := binary.BigEndian.Uint16(element.Key[family.size:])
				protocol.portStat[port]++

				stat, ok := family.stats[ip]
				if !ok {
					stat = &ipStat{}
					family.stats[ip] = stat
				}

				ipPorts := protocol.getPorts(stat)
				*ipPorts = append(*ipPorts, port)
			}
		}
	}

	if err := c.deleteBanned(ctx, toDelete); err != nil {
		return nil, err
	}

	toBan := make(map[string]struct{})

	for _, family := range addressFamilies {
		localStat := addressFamilyStat{label: "local"}
		remoteStat := addressFamilyStat{label: "remote"}

		for ipString, stat := range family.stats {
			ip := net.ParseIP(ipString)
			if ip == nil {
				return nil, fmt.Errorf("Got an invalid IP: %q", ipString)
			}

			isLocalAddress, isLocalNetwork := classifyAddress(localNetworks, ip)
			if isLocalAddress {
				continue
			}

			familyStat, tcpThreshold, udpThreshold := &remoteStat, remoteTCPPortScanThreshold, remoteUDPPortScanThreshold
			if isLocalNetwork {
				familyStat, tcpThreshold, udpThreshold = &localStat, localTCPPortScanThreshold, localUDPPortScanThreshold
			}

			familyStat.uniqueIPs++
			tcpScore := scorePortsUsage(stat.tcp, isLocalNetwork, scoreTCPPort)
			udpScore := scorePortsUsage(stat.udp, isLocalNetwork, scoreUDPPort)

			if tcpScore > tcpThreshold || udpScore > udpThreshold {
				logging.L(ctx).Warnf("%s port scan detected: %s: %s. TCP score: %d, UDP score: %d.",
					cases.Title(language.English).String(familyStat.label), ip, stat, tcpScore, udpScore)
				toBan[ip.String()] = struct{}{}
				continue
			}

			for _, protocol := range protocols {
				ports := len(*protocol.getPorts(stat))

				labels := metrics.NetworkLabels(family.label)
				labels[typeLabelName] = familyStat.label
				labels[protocolLabelName] = protocol.label
				inputConnectionsMetric.With(labels).Observe(float64(ports))

				if top := protocol.getTopStat(familyStat); ports > len(*protocol.getPorts(&top.stat)) {
					top.stat = *stat
					top.ip = mo.Some(ip)
				}
			}
		}

		for _, stat := range []*addressFamilyStat{&localStat, &remoteStat} {
			logging.L(ctx).Debugf("* Unique %s %s with new connection attempts: %d.",
				stat.label, family.name, stat.uniqueIPs)

			labels := metrics.NetworkLabels(family.label)
			labels[typeLabelName] = stat.label
			uniqueInputIPsMetric.With(labels).Set(float64(stat.uniqueIPs))

			for _, protocol := range protocols {
				top := protocol.getTopStat(stat)

				if ip, ok := top.ip.Get(); ok {
					ports := *protocol.getPorts(&top.stat)
					score := scorePortsUsage(ports, stat == &localStat, protocol.scorePort)
					logging.L(ctx).Debugf("* Top %s %s with new %s connection attempts: %s: %s. Score: %d.",
						stat.label, family.name, protocol.name, ip, top.stat, score)
				}

				labels := metrics.NetworkLabels(family.label)
				labels[typeLabelName] = stat.label
				labels[protocolLabelName] = protocol.label
				topInputIPMetric.With(labels).Set(float64(len(*protocol.getPorts(&top.stat))))
			}
		}
	}

	if logging.L(ctx).Level() <= zap.DebugLevel {
		for _, protocol := range protocols {
			var topPort mo.Option[uint16]
			var topCount int

			for port, count := range protocol.portStat {
				if count > topCount && protocol.scorePort(port, false) == unknownPortScore {
					topPort, topCount = mo.Some(port), count
				}
			}

			if port, ok := topPort.Get(); ok {
				logging.L(ctx).Debugf("* Most used %s port: %d (%d times).", protocol.name, port, topCount)
			}
		}
	}

	return toBan, nil
}

func (c *Collector) collectForwardIPs(ctx context.Context) error {
	logging.L(ctx).Debugf("Collecting forward IPs:")

	protocolType := nftables.TypeInetProto
	if size := protocolType.Bytes; size != 1 {
		return fmt.Errorf("Got an unexpected protocol data type size: %d", size)
	}
	protocolTypePadding := 4 - protocolType.Bytes

	portType := nftables.TypeInetService
	if size := portType.Bytes; size != 2 {
		return fmt.Errorf("Got an unexpected port data type size: %d", size)
	}
	portTypePadding := 4 - portType.Bytes

	stats := make(map[string]*forwardIPStat)

	for _, family := range getAddressFamilies() {
		expectedSize := family.dataType.Bytes + family.dataType.Bytes + protocolType.Bytes + protocolTypePadding + portType.Bytes + portTypePadding

		if size := family.dataType.Bytes; size != family.size {
			return fmt.Errorf("Got an unexpected %s data type size: %d", family.name, size)
		}

		elementType, err := nftables.ConcatSetType(family.dataType, family.dataType, protocolType, portType)
		if err != nil {
			return err
		} else if size := elementType.Bytes; size != expectedSize {
			return fmt.Errorf("Got an unexpected %s data type size: %d", elementType.Name, size)
		}

		setName := fmt.Sprintf("ip%d_forward_connections", family.version)

		_, elements, err := c.getSet(setName)
		if err != nil {
			return err
		}

		setSize := len(elements)
		forwardSetSizeMetric.With(metrics.NetworkLabels(family.label)).Set(float64(setSize))
		logging.L(ctx).Debugf("* %s forward connections set size: %d.", family.name, setSize)

		for _, element := range elements {
			element := element

			if size := len(element.Key); size != int(elementType.Bytes) {
				return fmt.Errorf(
					"Got %q set element of an unexpected size: %d vs %d",
					setName, size, elementType.Bytes)
			}

			var offset uint32
			getData := func(size uint32) []byte {
				data := element.Key[offset : offset+size]
				offset += size
				return data
			}

			ip := net.IP(getData(family.size)).String()
			_ = getData(family.size)

			protocol := getData(protocolType.Bytes + protocolTypePadding)[0]
			port := binary.BigEndian.Uint16(getData(portType.Bytes))

			stat, ok := stats[ip]
			if !ok {
				stat = newForwardIPStat(ip)
				stats[ip] = stat
			}

			switch protocol {
			case unix.IPPROTO_TCP:
				stat.tcp[port]++
			case unix.IPPROTO_UDP:
				stat.udp[port]++
			default:
				return fmt.Errorf("Got an unexpected protocol from %q set: %d", setName, protocol)
			}

			stat.total++
		}
	}

	var all []*forwardIPStat
	var top mo.Option[*forwardIPStat]
	debugMode := logging.L(ctx).Level() <= zap.DebugLevel

	for _, stat := range stats {
		if topStat, ok := top.Get(); !ok || topStat.total < stat.total {
			top = mo.Some(stat)
		}

		if debugMode {
			all = append(all, stat)
		}
	}

	if topIPStat, ok := top.Get(); ok {
		topForwardIPMetric.WithLabelValues().Set(float64(topIPStat.total))
	}

	if debugMode {
		sort.Slice(all, func(i, j int) bool {
			return all[i].total > all[j].total
		})

		for _, stat := range all {
			logging.L(ctx).Debugf("* %s", stat)
		}
	}

	return nil
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

func (c *Collector) getSet(name string) (*nftables.Set, []nftables.SetElement, error) {
	table, err := c.getTable()
	if err != nil {
		return nil, nil, err
	}

	set, err := c.connection.GetSetByName(table, name)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get %q set: %w", name, err)
	}

	elements, err := c.connection.GetSetElements(set)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to list %q set: %w", name, err)
	}

	return set, elements, nil
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
