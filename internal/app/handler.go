package app

import (
	"log"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
)

func (a *App) HandleConnection(conn transport.Connection) {
	defer conn.Close()
	peerSession := session.NewPeerSession(a.ctx, conn)

	peerSession.OnFileReceived(func(fileName string) {
		log.Printf("File received in HandleConnection %v\n", fileName)
		err := addNewTransferFileHistory(peerSession.GetPeerInfo().UserName, fileName, "RECEIVED", "COMPLETED")

		if err != nil {
			log.Printf("Error adding file history: %v\n", err)
		}
	})

	peerSession.OnDisconnected(func(peer discovery.Peer) {
		a.transferRegistry.PauseAllByPeerId(peer.ID)
		log.Printf("Peer disconnected %v\n", peer.Name)
	})

	peerSession.OnError(func(err error) {
		log.Printf("Session error %v\n", err)
	})

	peerSession.OnFileOffer(func(peerId string, fileId string, FileStatus transfer.Status, bytesDone int64) {
		log.Printf("Peer session on file offer in handler %v %v %v %v\n", peerId, fileId, FileStatus, bytesDone)
		a.transferRegistry.AddFileReceiving(peerId, fileId, FileStatus, bytesDone)
	})

	peerSession.Start(a.settings.DownloadPath)

	<-peerSession.Done()
}
