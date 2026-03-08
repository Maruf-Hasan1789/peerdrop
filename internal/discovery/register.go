package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/labstack/gommon/log"
)

func (d *Discovery) Register(ctx context.Context, selfPeer *Peer, userName string) error {
	time.Sleep(10 * time.Second)

	log.Printf("Started registering")
	userInfo := fmt.Sprintf("user_name=%s", userName)

	ifaces, err := getSecureLANInterfaces()

	if err != nil {
		return err
	}

	server, err := zeroconf.Register(
		selfPeer.Name,
		"_peerdrop._tcp",
		"local.",
		selfPeer.Port,
		[]string{"version=1", userInfo},
		ifaces,
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
