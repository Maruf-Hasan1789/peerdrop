package app

import (
	"log"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/domain"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	ChunkSize = 1024 * 1024
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
		err := peerSession.Stop()

		if err != nil {
			return
		}
		runtime.EventsEmit(a.ctx, "peer-disconnected", discovery.ToPeerDTO(peer))
		log.Printf("Peer disconnected %v\n", peer.Name)
		a.discovery.RemovePeerById(peer.ID)
	})

	peerSession.OnError(func(err error) {
		log.Printf("Session error %v\n", err)
	})

	peerSession.OnFileOffer(func(peerId string, fileName string, fileId string, status transfer.Status, chunkReceived int64, totalChunks int64) {
		log.Printf("Peer session on file offer in handler %v %v %v %v\n", peerId, fileId, status, totalChunks)
		a.transferRegistry.AddFileReceiving(peerId, fileId, fileName, status, chunkReceived, totalChunks)
		peer := peerSession.GetPeerInfo()
		senderInfo := discovery.SenderInfo{
			ID:       peerId,
			Name:     peer.Name,
			UserName: peer.UserName,
			Files: []domain.FileMetadata{{
				ID:       fileId,
				FileName: fileName,
				FileSize: totalChunks * ChunkSize,
			}},
		}

		a.mu.Lock()

		a.pendingPermissions[fileId] = pendingPermission{
			session: peerSession,
		}

		a.mu.Unlock()

		if a.settings.IsPermissionRequiredToSendFiles {
			runtime.EventsEmit(a.ctx, "permission-request", senderInfo)
		} else {
			a.ReceiveFilePermission(peerId, fileName, fileId, true)
		}
	})

	peerSession.Start(a.settings.DownloadPath)

	<-peerSession.Done()
}
