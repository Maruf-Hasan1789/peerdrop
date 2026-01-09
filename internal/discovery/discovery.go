package discovery

import "sync"

type Discovery struct {
	peers map[string]*Peer
	mu    sync.Mutex
}

func New() *Discovery {
	return &Discovery{
		peers: make(map[string]*Peer),
	}
}

func (d *Discovery) addOrUpdatePeer(p *Peer) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.peers[p.ID] = p
}
