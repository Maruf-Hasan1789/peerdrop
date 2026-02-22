package discovery

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/Maruf-Hasan1789/peerdrop/internal/utils"
	"github.com/grandcat/zeroconf"
	"github.com/labstack/gommon/log"
)

type ActivePeers struct {
	Peers []*Peer
}

type Peer struct {
	ID        string
	Name      string
	Addresses []net.IP
	Port      int
	Version   string
	UserName  string
}

type PeerDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
	Version   string   `json:"version"`
	UserName  string   `json:"user_name"`
}

func ToPeerDTO(p Peer) PeerDTO {
	addrs := make([]string, len(p.Addresses))
	for i, ip := range p.Addresses {
		addrs[i] = ip.String()
	}

	return PeerDTO{
		ID:        p.ID,
		Name:      p.Name,
		Addresses: addrs,
		Port:      p.Port,
		Version:   p.Version,
		UserName:  p.UserName,
	}
}

func NewPeerFromEntry(e *zeroconf.ServiceEntry) *Peer {
	log.Printf("NewPeerFromEntry %v\n", e.Text)
	userName := parseUserName(e.Text)

	if len(userName) == 0 {
		userName = e.Instance
	}
	return &Peer{
		ID:        peerID(e.Instance + e.Service + e.Domain),
		Name:      e.Instance,
		Addresses: append(e.AddrIPv4, e.AddrIPv6...),
		Port:      e.Port,
		Version:   parseVersion(e.Text),
		UserName:  userName,
	}
}

func parseUserName(text []string) string {
	for _, t := range text {
		if strings.HasPrefix(t, "user_name=") {
			return strings.TrimPrefix(t, "user_name=")
		}
	}

	return ""
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
	instanceName := fmt.Sprintf("peerdrop-%s", utils.GetHostname())
	serviceName := "_peerdrop._tcp"
	domain := "local."

	return &Peer{
		ID:        peerID(instanceName + serviceName + domain),
		Name:      instanceName,
		Addresses: utils.GetLocalIps(),
		Port:      port,
		Version:   parseVersion([]string{"version=1"}),
	}
}
