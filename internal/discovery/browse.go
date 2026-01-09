package discovery

import (
	"context"
	"log"
	"time"

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

func (d *Discovery) StartPersistentBrowse(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)

	defer ticker.Stop()

	for {
		scanCtx, cancel := context.WithTimeout(ctx, 15*time.Second)

		err := d.Browse(scanCtx)

		if err != nil {
			log.Printf("Browse error: %v\n", err)
		}
		cancel()

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}
