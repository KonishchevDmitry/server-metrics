package network

const localTCPPortScanThreshold = 10
const remoteTCPPortScanThreshold = 5

const localUDPPortScanThreshold = 10
const remoteUDPPortScanThreshold = 5

const allowedPortScore = 0
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
	case
		7,  // Android devices send echo requests to default gateway for some reason
		53: // DNS over TCP
		return localOnlyPort(local)

	case
		21,       // FTP
		23,       // Telnet
		25,       // SMTP
		110,      // POP3
		139, 445, // Samba
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
		3389,       // RDP
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
		5353,         // Multicast DNS
		32412, 32414: // Plex Player discovery
		return localOnlyPort(local)

	case
		53,       // DNS
		137, 138, // Samba
		5060: // Session Initiation Protocol (SIP)
		return portScanScore

	default:
		return unknownPortScore
	}
}

func localOnlyPort(local bool) int {
	if local {
		return allowedPortScore
	} else {
		return portScanScore
	}
}
