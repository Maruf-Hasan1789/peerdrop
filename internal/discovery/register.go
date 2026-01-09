package discovery

import (
	"context"
	"fmt"
	"log"

	"github.com/grandcat/zeroconf"
)

func (d *Discovery) Register(ctx context.Context, port int) error {
	log.Printf("Started registering")
	serviceName := fmt.Sprintf("peerdrop-%s-%d", hostname(), port)

	server, err := zeroconf.Register(
		serviceName,
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
