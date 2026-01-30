package discovery

import (
	"sync"

	"github.com/Maruf-Hasan1789/peerdrop/internal/domain"
	"github.com/labstack/gommon/log"
)

type Discovery struct {
	peers     map[string]*Peer
	mu        sync.Mutex
	observers []PeerObserver
}
type SenderInfo struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Addresses []string              `json:"addresses"`
	Port      string                `json:"port"`
	UserName  string                `json:"user_name"`
	Files     []domain.FileMetadata `json:"files"`
}

func New() *Discovery {
	return &Discovery{
		peers: make(map[string]*Peer),
	}
}

func (d *Discovery) addOrUpdatePeer(p *Peer) {
	log.Printf("Add or update Peer %v\n", p)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.peers[p.ID] = p
}

func (d *Discovery) GetPeers() []*Peer {
	d.mu.Lock()
	defer d.mu.Unlock()

	list := make([]*Peer, 0, len(d.peers))

	for _, p := range d.peers {
		list = append(list, p)
	}

	return list
}

func (d *Discovery) AddObserver(observer PeerObserver) {
	d.observers = append(d.observers, observer)
}

func (d *Discovery) NotifyOnPeerAdd(peer Peer) {
	for _, observer := range d.observers {
		go observer.OnPeerAdded(peer)
	}
}

func (d *Discovery) GetPeerById(peerId string) *Peer {
	log.Printf("GetPeerById %v\n", peerId)
	d.mu.Lock()
	defer d.mu.Unlock()

	p, ok := d.peers[peerId]
	if !ok {
		return nil
	}

	return p
}
