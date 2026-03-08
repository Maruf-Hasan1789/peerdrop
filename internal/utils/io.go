package utils

import (
	"log"
	"net"
	"os"
)

func GetLocalIps() []net.IP {
	var ips []net.IP
	var primaryIP net.IP

	ifaces, err := net.Interfaces()

	if err != nil {
		log.Fatal("Error while getting local ips")
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()

		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			if ip.IsLinkLocalUnicast() || ip.IsLoopback() {
				continue
			}

			if primaryIP == nil && ip.To4() != nil {
				primaryIP = ip
			}
			ips = append(ips, ip)
		}
	}

	if primaryIP != nil {
		for i, ip := range ips {
			if ip.Equal(primaryIP) {
				ips = append(ips[:i], ips[i+1:]...)
				break
			}
		}

		ips = append([]net.IP{primaryIP}, ips...)
	}

	return ips
}

func GetHostname() string {
	name, err := os.Hostname()

	if err != nil {
		return "unknown"
	}

	return name
}
