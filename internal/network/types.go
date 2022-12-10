package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/samber/mo"

	"github.com/google/nftables"
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
	name       string
	label      string
	getStat    func(stat *ipStat) *int
	getTopStat func(stat *addressFamilyStat) *topIPStat
}

func makeProtocolFamily(
	name string, getStat func(stat *ipStat) *int,
	getTopStat func(stat *addressFamilyStat) *topIPStat,
) protocolFamily {
	return protocolFamily{
		name:       name,
		label:      strings.ToLower(name),
		getStat:    getStat,
		getTopStat: getTopStat,
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
	tcp int
	udp int
}

func (s ipStat) String() string {
	return fmt.Sprintf("%d TCP, %d UDP", s.tcp, s.udp)
}
