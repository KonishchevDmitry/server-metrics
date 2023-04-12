package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/google/nftables"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/mo"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

type Collector struct {
	lock   sync.Mutex
	logger *zap.SugaredLogger

	connection *nftables.Conn
	table      mo.Option[*nftables.Table]

	dryRun bool
	banned map[string]struct{}
}

var _ prometheus.Collector = &Collector{}

func NewCollector(logger *zap.SugaredLogger, dryRun bool) (retCollector *Collector, retErr error) {
	connection, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return nil, fmt.Errorf("Unable to open netlink connection: %w", err)
	}
	return &Collector{
		logger:     logger,
		connection: connection,
		dryRun:     dryRun,
	}, nil
}

func (c *Collector) Close(ctx context.Context) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.connection != nil {
		if err := c.connection.CloseLasting(); err != nil {
			logging.L(ctx).Errorf("Failed to close netlink connection: %s.", err)
		}
		c.connection = nil
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- uniqueInputIPsMetric
	inputConnectionsMetric().Describe(descs)
	descs <- topInputIPMetric
	descs <- topForwardIPMetric
	descs <- inputSetSizeMetric
	descs <- forwardSetSizeMetric
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.connection == nil {
		return
	}

	ctx := logging.WithLogger(context.Background(), c.logger)

	toBan, err := c.collectInputIPs(ctx, c.banned, metrics)
	if err == nil {
		err = c.collectForwardIPs(ctx, metrics)
	}
	if err != nil {
		logging.L(ctx).Errorf("Failed to collect network metrics: %s.", err)
	}

	if !c.dryRun {
		// Give a time to fail2ban to react on the log message and remove the set elements on next iteration
		c.banned = toBan
	}
}

func (c *Collector) collectInputIPs(
	ctx context.Context, banned map[string]struct{}, metrics chan<- prometheus.Metric,
) (map[string]struct{}, error) {
	logging.L(ctx).Debugf("Collecting input IPs:")

	addressFamilies := getAddressFamilies()
	protocols := getProtocolFamilies()

	inputConnections := inputConnectionsMetric()
	defer inputConnections.Collect(metrics)

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

			metrics <- prometheus.MustNewConstMetric(
				inputSetSizeMetric, prometheus.GaugeValue, float64(setSize),
				family.label, protocol.label)

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
				inputConnections.WithLabelValues(family.label, familyStat.label, protocol.label).Observe(float64(ports))

				if top := protocol.getTopStat(familyStat); ports > len(*protocol.getPorts(&top.stat)) {
					top.stat = *stat
					top.ip = mo.Some(ip)
				}
			}
		}

		for _, stat := range []*addressFamilyStat{&localStat, &remoteStat} {
			logging.L(ctx).Debugf("* Unique %s %s with new connection attempts: %d.",
				stat.label, family.name, stat.uniqueIPs)

			metrics <- prometheus.MustNewConstMetric(
				uniqueInputIPsMetric, prometheus.GaugeValue, float64(stat.uniqueIPs),
				family.label, stat.label)

			for _, protocol := range protocols {
				top := protocol.getTopStat(stat)

				if ip, ok := top.ip.Get(); ok {
					ports := *protocol.getPorts(&top.stat)
					score := scorePortsUsage(ports, stat == &localStat, protocol.scorePort)
					logging.L(ctx).Debugf("* Top %s %s with new %s connection attempts: %s: %s. Score: %d.",
						stat.label, family.name, protocol.name, ip, top.stat, score)
				}

				metrics <- prometheus.MustNewConstMetric(
					topInputIPMetric, prometheus.GaugeValue, float64(len(*protocol.getPorts(&top.stat))),
					family.label, stat.label, protocol.label)
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

func (c *Collector) collectForwardIPs(ctx context.Context, metrics chan<- prometheus.Metric) error {
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
	debugMode := logging.L(ctx).Level() <= zap.DebugLevel

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
		metrics <- prometheus.MustNewConstMetric(
			forwardSetSizeMetric, prometheus.GaugeValue, float64(setSize),
			family.label)

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
				name := ip

				if debugMode {
					names, err := net.LookupAddr(ip)
					if err != nil {
						logging.L(ctx).Errorf("Failed to lookup %s: %s.", ip, err)
					}
					if len(names) != 0 {
						name = fmt.Sprintf("%s (%s)", strings.TrimSuffix(names[0], "."), ip)
					}
				}

				stat = newForwardIPStat(name)
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

	for _, stat := range stats {
		if topStat, ok := top.Get(); !ok || topStat.total < stat.total {
			top = mo.Some(stat)
		}

		if debugMode {
			all = append(all, stat)
		}
	}

	if topIPStat, ok := top.Get(); ok {
		metrics <- prometheus.MustNewConstMetric(
			topForwardIPMetric, prometheus.GaugeValue, float64(topIPStat.total))
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
		if deletedElements != 0 {
			logging.L(ctx).Infof("%d elements have been deleted from ports connections tracking sets.", deletedElements)
		}
	}()

	// The maximum netlink message size is limited by socket send buffer, so don't delete too much in one call
	const batchSize = 1000

	for set, elements := range sets {
		for len(elements) != 0 {
			count := batchSize
			if elements := len(elements); count > elements {
				count = elements
			}

			err := c.connection.SetDeleteElements(set, elements[:count])
			if err == nil {
				err = c.connection.Flush()
			}
			if err != nil {
				return fmt.Errorf("Unable to delete %d elements from %q set: %w", count, set.Name, err)
			}

			deletedElements += count
			elements = elements[count:]
		}
	}

	return nil
}
