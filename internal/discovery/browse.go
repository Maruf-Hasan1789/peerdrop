package discovery

import (
	"context"
	"log"

	"github.com/grandcat/zeroconf"
)

func (d *Discovery) Browse(ctx context.Context) error {
	log.Printf("Started browsing")
	resolver, _ := zeroconf.NewResolver(nil)

	entries := make(chan *zeroconf.ServiceEntry)

	go d.consumeEntries(ctx, entries)

	return resolver.Browse(ctx, "_peerdrop._tcp", "local.", entries)
}

func (d *Discovery) consumeEntries(ctx context.Context, entries <-chan *zeroconf.ServiceEntry) {

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-entries:
			if !ok {
				return
			}

			peer := NewPeerFromEntry(e)
			log.Printf("New  Peer: %v %v", peer.Name, peer.Port)
			d.addOrUpdatePeer(peer)
		}
	}
}
