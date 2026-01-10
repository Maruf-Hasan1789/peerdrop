package discovery

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/Maruf-Hasan1789/peerdrop/internal/utils"
	"github.com/grandcat/zeroconf"
)

type Peer struct {
	ID        string
	Name      string
	Addresses []net.IP
	Port      int
	Version   string
}

func NewPeerFromEntry(e *zeroconf.ServiceEntry) *Peer {
	return &Peer{
		ID:        peerID(e.Instance),
		Name:      e.Instance,
		Addresses: append(e.AddrIPv4, e.AddrIPv6...),
		Port:      e.Port,
		Version:   parseVersion(e.Text),
	}
}

func peerID(instanceName string) string {
	h := sha1.Sum([]byte(instanceName))

	return hex.EncodeToString(h[:8])
}

func parseVersion(txt []string) string {
	for _, t := range txt {
		if strings.HasPrefix(t, "version=") {
			return strings.TrimPrefix(t, "version=")
		}
	}

	return "unknown"
}

func GetSelfPeer(port int) *Peer {
	serviceName := fmt.Sprintf("peerdrop-%s-%d", utils.GetHostname(), port)
	return &Peer{
		ID:        peerID(serviceName),
		Name:      serviceName,
		Addresses: utils.GetLocalIps(),
		Port:      port,
		Version:   parseVersion([]string{"version=1"}),
	}
}
