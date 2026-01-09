package discovery

import (
	"crypto/sha1"
	"encoding/hex"
	"net"
	"strings"

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
		ID:        peerID(e),
		Name:      e.Instance,
		Addresses: append(e.AddrIPv4, e.AddrIPv6...),
		Port:      e.Port,
		Version:   parseVersion(e.Text),
	}
}

func peerID(e *zeroconf.ServiceEntry) string {
	h := sha1.Sum([]byte(e.Instance))

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
