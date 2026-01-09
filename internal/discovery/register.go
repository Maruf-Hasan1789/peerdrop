package discovery

import (
	"context"
	"fmt"
	"log"

	"github.com/grandcat/zeroconf"
)

func (d *Discovery) Register(ctx context.Context, port int) error {
	fmt.Printf("My Port: %v\n", port)
	log.Printf("Started registering")

	server, err := zeroconf.Register(
		"PeerDrop-"+hostname(),
		"_peerdrop._tcp",
		"local.",
		port,
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
