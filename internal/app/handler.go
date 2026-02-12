package app

import (
	"os"
	"path/filepath"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/domain"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/labstack/gommon/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	ChunkSize = 1024 * 1024
)

func (a *App) HandleConnection(conn transport.Connection) {
	defer conn.Close()
	peerSession := session.NewPeerSession(a.ctx, conn)

	peerSession.OnFileReceived(func(peerId string, rootId string, rootName string) {
		log.Printf("File received in HandleConnection %v\n", rootName)
		//need to change to rootId
		a.transferRegistry.RemoveFileRegistryUponCompletion(peerId, rootId, transfer.Incoming)
		err := addNewTransferFileHistory(peerSession.GetPeerInfo().UserName, rootName, "RECEIVED", "COMPLETED", "")

		if err != nil {
			log.Printf("Error adding file history: %v\n", err)
		}
	})

	peerSession.OnDisconnected(func(peer discovery.Peer) {
		log.Info("peer disconnected %v %v\n", peer.Name, peer.UserName)
		if a.ctx != nil {
			a.transferRegistry.PauseAllByPeerId(peer.ID)
			log.Printf("Emitting Events\n")
			a.discovery.RemovePeerById(peer.ID)

			transferRegistry := a.transferRegistry.GetAllTransfersByPeerId(peer.ID)

			for _, t := range transferRegistry {
				if t.PeerId == peer.ID && t.Status == transfer.Paused && t.Direction == transfer.Incoming {
					log.Printf("transfer %v has been paused\n", t)
					cleanUpPartialDownload(a.settings.DownloadPath, t.RootName)
					err := addNewTransferFileHistory(peer.UserName, t.RootName, "RECEIVED", "FAILED", t.TransferId)
					if err != nil {
						log.Printf("Error adding file history: %v", err)
					}

					runtime.EventsEmit(a.ctx, "receiving-failed", map[string]interface{}{
						"peerId": peer.ID,
						"Id":     t.RootID,
						"file":   t.RootName,
					})
				}
			}

			runtime.EventsEmit(a.ctx, "peer-disconnected", map[string]interface{}{
				"user_name": peer.UserName,
				"id":        peer.ID,
				"port":      peer.Port,
			})

			err := peerSession.Stop()
			if err != nil {
				log.Printf("Error stopping peer: %v", err)
				return
			}
		}
	})

	peerSession.OnError(func(err error) {
		log.Printf("Session error %v\n", err)
	})

	peerSession.OnFileOffer(func(peerId string, transferId string, rootEntries []*protocol.RootEntry, totalSize int64) {
		log.Printf("Peer session on file offer in handler %v %v %v\n", peerId, rootEntries, totalSize)

		var newFiles []domain.FileMetadata

		for _, rootEntry := range rootEntries {
			newFile := domain.FileMetadata{
				ID:       rootEntry.ID,
				FileName: rootEntry.Name,
				FileSize: rootEntry.Size,
			}

			a.transferRegistry.AddTransferredFile(peerId, transferId, rootEntry.ID, rootEntry.Name, transfer.Pending, 0, rootEntry.Size, transfer.Incoming)
			newFiles = append(newFiles, newFile)
		}

		peer := peerSession.GetPeerInfo()

		senderInfo := discovery.SenderInfo{
			ID:         peerId,
			TransferId: transferId,
			Name:       peer.Name,
			UserName:   peer.UserName,
			Files:      newFiles,
		}

		a.mu.Lock()

		a.pendingPermissions[transferId] = pendingPermission{
			session: peerSession,
		}

		a.mu.Unlock()

		if a.settings.IsPermissionRequiredToSendFiles {
			runtime.EventsEmit(a.ctx, "permission-request", senderInfo)
		} else {
			maap := make(map[string]bool)

			for _, file := range newFiles {
				maap[file.ID] = true
			}

			//allowing all files
			a.ReceiveFilePermission(peerId, transferId, maap, true)
		}
	})

	peerSession.Start(a.settings.DownloadPath)

	<-peerSession.Done()
}

func cleanUpPartialDownload(downloadPath string, fileName string) {
	filePath := filepath.Join(downloadPath, fileName)
	if _, err := os.Stat(filePath); err == nil {
		err := os.Remove(filePath)
		if err != nil {
			log.Printf("Error removing file %v: %v\n", filePath, err)
		}
	}
}
