package discovery

import (
	"context"
	"log"
	"time"

	"github.com/grandcat/zeroconf"
)

func (d *Discovery) Register(ctx context.Context, selfPeer *Peer) error {
	time.Sleep(10 * time.Second)
	log.Printf("Started registering")

	server, err := zeroconf.Register(
		selfPeer.Name,
		"_peerdrop._tcp",
		"local.",
		selfPeer.Port,
		[]string{"version=1"},
		nil,
	)

	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()

	return nil
}
