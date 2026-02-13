package discovery

import (
	"fmt"
	"net"
	"strings"

	"github.com/labstack/gommon/log"
)

func getSecureLANInterfaces() ([]net.Interface, error) {
	var result []net.Interface

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {

		// Must be up
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Must support multicast
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}

		// Skip VPN / virtual (name-based)
		name := strings.ToLower(iface.Name)
		if strings.Contains(name, "vpn") ||
			strings.Contains(name, "tun") ||
			strings.Contains(name, "tap") ||
			strings.Contains(name, "wireguard") ||
			strings.Contains(name, "wg") ||
			strings.Contains(name, "nord") ||
			strings.Contains(name, "tailscale") ||
			strings.Contains(name, "zerotier") ||
			strings.Contains(name, "docker") ||
			strings.Contains(name, "vm") ||
			strings.Contains(name, "virtual") ||
			strings.Contains(name, "hyper") {
			continue
		}

		// Must have private IPv4
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}

			if ip.To4() != nil && ip.IsPrivate() {
				result = append(result, iface)
				break
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no secure LAN interfaces found")
	}

	log.Printf("Secure Lan interfaces found: %v\n", result)
	return result, nil
}
