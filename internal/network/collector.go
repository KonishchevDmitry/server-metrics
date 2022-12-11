package network

import (
	"context"
	"fmt"
	"net"

	"github.com/google/nftables"
	"github.com/samber/mo"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
)

// FIXME(konishchev): Alter it
const localTCPPortScanThreshold = 100
const localUDPPortScanThreshold = 100

const remoteTCPPortScanThreshold = 5
const remoteUDPPortScanThreshold = 10

const typeLabelName = "type"
const protocolLabelName = "protocol"

var uniqueIPsMetric = metrics.NetworkMetric(
	"new_connections", "ips", "Count of IP addresses with new connection attempts.",
	typeLabelName)

var topIPMetric = metrics.NetworkMetric(
	"port_connections", "top_ip", "Count of unique ports with new connections attempts for the top IP.",
	typeLabelName, protocolLabelName)

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

	protocols := []protocolFamily{
		makeProtocolFamily(
			"TCP", func(stat *ipStat) *int { return &stat.tcp },
			func(stat *addressFamilyStat) *topIPStat { return &stat.topTCP }),

		makeProtocolFamily(
			"UDP", func(stat *ipStat) *int { return &stat.udp },
			func(stat *addressFamilyStat) *topIPStat { return &stat.topUDP }),
	}

	toDelete := make(map[*nftables.Set][]nftables.SetElement)

	table, err := c.getTable()
	if err != nil {
		return nil, err
	}

	localNetworks, err := getLocalNetworks()
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

		for _, protocol := range protocols {
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

				*protocol.getStat(stat)++
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
			if stat.tcp >= tcpThreshold || stat.udp >= udpThreshold {
				logging.L(ctx).Warnf("%s port scan detected: %s: %s.",
					cases.Title(language.English).String(familyStat.label), ip, stat)
				toBan[ip.String()] = struct{}{}
				continue
			}

			if top := &familyStat.topTCP; stat.tcp > top.stat.tcp {
				top.stat = *stat
				top.ip = mo.Some(ip)
			}

			if top := &familyStat.topUDP; stat.udp > top.stat.udp {
				top.stat = *stat
				top.ip = mo.Some(ip)
			}
		}

		for _, stat := range []*addressFamilyStat{&localStat, &remoteStat} {
			logging.L(ctx).Debugf("Unique %s %s with new connection attempts: %d.",
				stat.label, family.name, stat.uniqueIPs)

			labels := metrics.NetworkLabels(family.label)
			labels[typeLabelName] = stat.label
			uniqueIPsMetric.With(labels).Set(float64(stat.uniqueIPs))

			for _, protocol := range protocols {
				top := protocol.getTopStat(stat)
				if ip, ok := top.ip.Get(); ok {
					logging.L(ctx).Debugf("Top %s with new %s connection attempts: %s: %s.",
						family.name, protocol.name, ip, top.stat)
				}

				labels := metrics.NetworkLabels(family.label)
				labels[typeLabelName] = stat.label
				labels[protocolLabelName] = protocol.label
				topIPMetric.With(labels).Set(float64(*protocol.getStat(&top.stat)))
			}
		}
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
