package network

import (
	"fmt"
	"net"
)

func getLocalNetworks() ([]*net.IPNet, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("Failed to get network interfaces: %w", err)
	}

	var networks []*net.IPNet

	for _, iface := range ifaces {
		addresses, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("Failed to get addresses of %q network interface: %w", iface.Name, err)
		}

		for _, address := range addresses {
			network, ok := address.(*net.IPNet)
			if !ok {
				return nil, fmt.Errorf("Got an unexpected IP address type (%T): %s", address, address)
			}
			networks = append(networks, network)
		}
	}

	return networks, nil
}

func classifyAddress(localNetworks []*net.IPNet, ip net.IP) (bool, bool) {
	isLocalAddress := ip.IsLoopback()

	isLocalNetwork := isLocalAddress
	if ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		isLocalNetwork = true
	}

	for _, network := range localNetworks {
		if network.IP.Equal(ip) {
			isLocalAddress, isLocalNetwork = true, true
			break
		}

		if network.Contains(ip) {
			isLocalNetwork = true
		}
	}

	return isLocalAddress, isLocalNetwork
}
