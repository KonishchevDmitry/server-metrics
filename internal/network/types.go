package network

import (
	"fmt"
	"net"
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
