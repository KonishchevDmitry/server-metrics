package network

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/google/nftables"
	"github.com/samber/mo"

	"github.com/KonishchevDmitry/server-metrics/internal/util"
)

type addressFamily struct {
	version int
	name    string
	label   string

	size     uint32
	dataType nftables.SetDatatype

	stats map[string]*ipStat
}

func getAddressFamilies() []*addressFamily {
	return []*addressFamily{
		newAddressFamily(4, net.IPv4len, nftables.TypeIPAddr),
		newAddressFamily(6, net.IPv6len, nftables.TypeIP6Addr),
	}
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

	getPorts   func(stat *ipStat) *[]uint16
	getTopStat func(stat *addressFamilyStat) *topIPStat
	scorePort  scorePortFunc

	portStat map[uint16]int
}

func getProtocolFamilies() []*protocolFamily {
	return []*protocolFamily{
		newProtocolFamily(
			"TCP", func(stat *ipStat) *[]uint16 { return &stat.tcp },
			func(stat *addressFamilyStat) *topIPStat { return &stat.topTCP },
			scoreTCPPort),

		newProtocolFamily(
			"UDP", func(stat *ipStat) *[]uint16 { return &stat.udp },
			func(stat *addressFamilyStat) *topIPStat { return &stat.topUDP },
			scoreUDPPort),
	}
}

func newProtocolFamily(
	name string, getPorts func(stat *ipStat) *[]uint16, getTopStat func(stat *addressFamilyStat) *topIPStat,
	scorePort scorePortFunc,
) *protocolFamily {
	return &protocolFamily{
		name:  name,
		label: strings.ToLower(name),

		getPorts:   getPorts,
		getTopStat: getTopStat,
		scorePort:  scorePort,

		portStat: make(map[uint16]int),
	}
}

type addressFamilyStat struct {
	label     string
	uniqueIPs int
	topTCP    topIPStat
	topUDP    topIPStat
}

type topIPStat struct {
	ip   mo.Option[net.IP]
	stat ipStat
}

type ipStat struct {
	tcp []uint16
	udp []uint16
}

func (s ipStat) String() string {
	return fmt.Sprintf("%d TCP (%s), %d UDP (%s)",
		len(s.tcp), util.FormatList(s.tcp, true), len(s.udp), util.FormatList(s.udp, true))
}

type forwardIPStat struct {
	name  string
	tcp   map[uint16]int
	udp   map[uint16]int
	total int
}

func newForwardIPStat(name string) *forwardIPStat {
	return &forwardIPStat{
		name: name,
		tcp:  make(map[uint16]int),
		udp:  make(map[uint16]int),
	}
}

func (s *forwardIPStat) String() string {
	var buf bytes.Buffer

	_, _ = fmt.Fprintf(&buf, "%s (%d IP+port pairs): ", s.name, s.total)
	formatPortStat(&buf, "TCP", s.tcp)
	buf.WriteString(", ")
	formatPortStat(&buf, "UDP", s.udp)

	return buf.String()
}

func formatPortStat(buf *bytes.Buffer, name string, ports map[uint16]int) {
	type portStat struct {
		port  uint16
		count int
	}

	var total int
	stats := make([]portStat, 0, len(ports))

	for port, count := range ports {
		stats = append(stats, portStat{
			port:  port,
			count: count,
		})
		total += count
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].count > stats[j].count
	})

	_, _ = fmt.Fprintf(buf, "%d %s", total, name)

	if len(stats) != 0 {
		buf.WriteString(" (")

		for index, stat := range stats {
			if index != 0 {
				buf.WriteString(", ")
			}
			_, _ = fmt.Fprintf(buf, "%dx%d", stat.count, stat.port)
		}

		buf.WriteByte(')')
	}
}
