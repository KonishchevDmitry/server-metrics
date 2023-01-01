package network

// Threshold is bigger for UDP, because there are chances of false positives for forwarded packets due to conntrack
// expiration, which is not the case for TCP.

const localTCPPortScanThreshold = 10
const remoteTCPPortScanThreshold = 5

// FIXME(konishchev): Conntrack is too unreliable for UDP. Drop it?
const localUDPPortScanThreshold = 1000000000
const remoteUDPPortScanThreshold = 1000000000

const publicPortScore = 0
const unknownPortScore = 1
const portScanScore = 3

type scorePortFunc func(port uint16, local bool) int

func scorePortsUsage(ports []uint16, local bool, scoreFunc scorePortFunc) int {
	var score int
	for _, port := range ports {
		score += scoreFunc(port, local)
	}
	return score
}

func scoreTCPPort(port uint16, local bool) int {
	switch port {
	case 22, 80, 443:
		return publicPortScore

	case
		7,        // Android devices send echo requests to default gateway for some reason
		53,       // DNS over TCP
		139, 445, // Samba
		2222,        // VM SSH
		3389,        // VM RDP
		6771,        // Transmission LSD
		8384, 22000, // Syncthing
		32400: // Plex
		return localOnlyPort(local)

	case
		21,         // FTP
		23,         // Telnet
		25,         // SMTP
		110,        // POP3
		143,        // IMAP
		389,        // LDAP
		465,        // SMTP + TLS
		500,        // IPSec
		631,        // Internet Printing Protocol
		993,        // IMAP + TLS
		995,        // POP3 + TLS
		1433,       // MSSQL
		2376,       // Docker REST API
		3128,       // Proxy
		3306,       // MySQL
		4899,       // Radmin
		5432,       // PostgreSQL
		5900, 5901, // VNC
		6443,                         // Kubernetes API
		6881,                         // BitTorrent trackers/clients
		8000, 8008, 8080, 8443, 8888, // Devel ports
		10250: // Kubelet API
		return portScanScore

	default:
		return unknownPortScore
	}
}

func scoreUDPPort(port uint16, local bool) int {
	switch port {
	case
		1194,  // OpenVPN
		10101, // WireGuard

		60000, 60001, 60002, 60003, 60004, 60005, 60006, 60007, 60008, 60009, 60010: // Mosh
		return publicPortScore

	case
		53,     // DNS
		67, 68, // DHCP
		137, 138, // Samba
		5353,         // Multicast DNS
		21027, 22000, // Syncthing
		32412, 32414: // Plex Player discovery
		return localOnlyPort(local)

	case 5060: // Session Initiation Protocol (SIP)
		return portScanScore

	default:
		return unknownPortScore
	}
}

func localOnlyPort(local bool) int {
	if local {
		return publicPortScore
	} else {
		return portScanScore
	}
}
