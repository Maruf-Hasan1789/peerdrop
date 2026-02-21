package app

import (
	"os"
	"path/filepath"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/domain"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport2 "github.com/Maruf-Hasan1789/peerdrop/internal/transport"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/labstack/gommon/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) HandleConnection(conn transport.Connection) {
	defer conn.Close()
	config := transport2.DefaultConfig()
	peerSession := session.NewPeerSession(a.ctx, conn, config)

	peerSession.OnFileReceived(func(peerId string, rootId string, rootName string) {
		log.Printf("File received in HandleConnection %v\n", rootName)

		a.transferRegistry.RemoveFileRegistryUponCompletion(peerId, rootId)
		err := addNewTransferFileHistory(peerSession.GetPeerInfo().UserName, rootName, "RECEIVED", "COMPLETED", rootId)

		if err != nil {
			log.Printf("Error adding file history: %v\n", err)
		}

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "file-received", map[string]interface{}{
				"id":   rootId,
				"file": rootName,
				"peer": peerSession.GetPeerInfo().UserName,
			})
		}
	})

	peerSession.OnDisconnected(func(peer discovery.Peer) {
		log.Printf("Here in handler go line 45\n")
		log.Info("peer disconnected %v %v\n", peer.Name, peer.UserName)
		if a.ctx != nil {
			//a.transferRegistry.PauseAllByPeerId(peer.ID)
			log.Printf("Emitting Events\n")

			transferRegistry := a.transferRegistry.GetAllTransfersByPeerId(peer.ID)

			for _, t := range transferRegistry {
				if t.Meta.PeerId == peer.ID && (t.Meta.Status == transfer.InProgress || t.Meta.Status == transfer.Pending) && t.Meta.Direction == transfer.Incoming {
					log.Printf("transfer %v has been cancelled\n", t)
					cleanUpPartialDownload(a.settings.DownloadPath, t.Meta.RootName)
					err := addNewTransferFileHistory(peer.UserName, t.Meta.RootName, "RECEIVED", "FAILED", t.Meta.RootID)
					if err != nil {
						log.Printf("Error adding file history: %v", err)
					}

					runtime.EventsEmit(a.ctx, "receiving-failed", map[string]interface{}{
						"peerId": peer.ID,
						"Id":     t.Meta.RootID,
						"file":   t.Meta.RootName,
					})
				}
			}

			log.Printf("Peer Info %v\n", peer)

			a.discovery.RemovePeerById(peer.ID)

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

			a.transferRegistry.AddTransferredFile(a.ctx, peerId, transferId, rootEntry.ID, rootEntry.Name, transfer.Pending, 0, rootEntry.Size, transfer.Incoming)
			newFiles = append(newFiles, newFile)
			registry := a.transferRegistry.GetTransferRegistryByRootId(rootEntry.ID)

			log.Printf("Transfer Registry: %v %v\n", (*registry).Meta.RootID, (*registry).Meta.Status)
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
			log.Printf("Sender Info transfer ID %v", transferId)
			runtime.EventsEmit(a.ctx, "permission-request", senderInfo)
		} else {
			maap := make(map[string]bool)

			for _, file := range newFiles {
				maap[file.ID] = true
			}

			//allowing all files
			permissionResp := FileReceivePermissionResponse{
				Mode:  protocol.PermissionAll,
				Files: maap,
			}

			a.ReceiveFilePermission(peerId, transferId, permissionResp)
		}
	})

	peerSession.Start(a.settings.DownloadPath)

	<-peerSession.Done()
}

func cleanUpPartialDownload(downloadPath string, fileName string) {
	log.Printf("Here in clean up partial download")
	filePath := filepath.Join(downloadPath, fileName)
	if _, err := os.Stat(filePath); err == nil {
		err := os.RemoveAll(filePath)
		if err != nil {
			log.Printf("Error removing file %v: %v\n", filePath, err)
		}
	}
}
