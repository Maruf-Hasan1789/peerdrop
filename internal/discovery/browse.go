package discovery

import (
	"context"
	"log"
	"time"

	"github.com/grandcat/zeroconf"
)

func (d *Discovery) Browse(ctx context.Context, selfPeer Peer) error {
	time.Sleep(10 * time.Second)

	log.Printf("Started browsing")

	ifaces, err := getSecureLANInterfaces()

	if err != nil {
		log.Printf("Error getting secure LAN interfaces: %v", err)
		return err
	}

	resolver, _ := zeroconf.NewResolver(zeroconf.SelectIfaces(ifaces))

	entries := make(chan *zeroconf.ServiceEntry)

	go d.consumeEntries(ctx, entries, selfPeer)

	return resolver.Browse(ctx, "_peerdrop._tcp", "local.", entries)
}

func (d *Discovery) consumeEntries(ctx context.Context, entries <-chan *zeroconf.ServiceEntry, selfPeer Peer) {

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-entries:
			if !ok {
				log.Printf("Browsing stopped\n")
				return
			}

			peer := NewPeerFromEntry(e)

			if peer.ID == selfPeer.ID {
				continue
			}

			d.addOrUpdatePeer(peer)
			d.NotifyOnPeerAdd(*peer)
		}
	}
}
