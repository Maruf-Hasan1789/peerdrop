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

func (d *Discovery) GetPeers() []*Peer {
	d.mu.Lock()
	defer d.mu.Unlock()

	list := make([]*Peer, 0, len(d.peers))

	for _, p := range d.peers {
		list = append(list, p)
	}

	return list
}
