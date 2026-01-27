package app

import (
	"log"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
)

func (a *App) HandleConnection(conn transport.Connection) {
	defer conn.Close()
	peerSession := session.NewPeerSession(a.ctx, conn)

	peerSession.OnFileReceived(func(name string) {
		log.Printf("File received %v\n", name)
	})

	peerSession.OnDisconnected(func(peer discovery.Peer) {
		log.Printf("Peer disconnected %v\n", peer.Name)
	})

	peerSession.OnError(func(err error) {
		log.Printf("Session error %v\n", err)
	})

	peerSession.Start(a.settings.DownloadPath)

	<-peerSession.Done()
}
