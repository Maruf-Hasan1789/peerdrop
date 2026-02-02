package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/labstack/gommon/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                context.Context
	discovery          *discovery.Discovery
	settings           *Settings
	permissionManager  *PermissionManager
	transferRegistry   *transfer.Registry
	pendingPermissions map[string]pendingPermission
	mu                 sync.Mutex
}

type pendingPermission struct {
	session *session.PeerSession
}

func NewApp(d *discovery.Discovery, registry *transfer.Registry) *App {
	return &App{
		discovery:          d,
		transferRegistry:   registry,
		pendingPermissions: make(map[string]pendingPermission),
	}
}

func (a *App) Startup(ctx context.Context, settings *Settings) {
	a.ctx = ctx
	a.settings = settings
	//a.permissionManager = permissionManager
	_, err := loadOrCreateTransferHistory()

	if err != nil {
		log.Printf("Error loading sent transfer history: %v", err)
	}
}

func (a *App) Name(name string) string {
	log.Printf("Random event from go")
	runtime.EventsEmit(a.ctx, "randomEvent", name)
	return name
}

func (a *App) bindSession(p *session.PeerSession) {
	p.OnFileReceived(func(name string) {
		log.Printf("File received: in Bind Session %s", name)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "file-received", map[string]interface{}{
				"name": name,
				"peer": p.GetPeerInfo().UserName,
			})
		}
	})

	p.OnDisconnected(func(peer discovery.Peer) {
		log.Printf("peer disconnected %v %v\n", peer.Name, peer.UserName)
		if a.ctx != nil {
			a.transferRegistry.PauseAllByPeerId(peer.ID)
			log.Printf("Emitting Events\n")
			a.discovery.RemovePeerById(peer.ID)

			transferRegistry := a.transferRegistry.GetAllTransfersByPeerId(peer.ID)

			for _, t := range transferRegistry {
				if t.PeerId == peer.ID && t.Status == transfer.Paused {

					err := addNewTransferFileHistory(peer.UserName, t.FileName, "SENT", "FAILED")
					if err != nil {
						log.Printf("Error adding file history: %v", err)
					}

				}
			}

			runtime.EventsEmit(a.ctx, "peer-disconnected", map[string]interface{}{
				"user_name": peer.UserName,
				"id":        peer.ID,
				"port":      peer.Port,
			})

			err := p.Stop()
			if err != nil {
				log.Printf("Error stopping peer: %v", err)
				return
			}
		}

	})
}

var peerSessions = make(map[string]*session.PeerSession)

func (a *App) RegisterSession(peerSession *session.PeerSession) {
	peerSessions[peerSession.GetPeerInfo().ID] = peerSession
	a.bindSession(peerSession)
}

func (a *App) ListPeers() []discovery.PeerDTO {
	peers := a.discovery.GetPeers() // internal Peer
	result := make([]discovery.PeerDTO, 0, len(peers))

	for _, p := range peers {
		result = append(result, discovery.ToPeerDTO(*p))
	}
	log.Info("Here Peer in List Peer ", result)
	return result
}

func (a *App) OnPeerAdded(peer discovery.Peer) {
	// 🔥 Notify frontend immediately
	log.Printf("On Peer Added 97 %v\n", peer.Name)
	if a.ctx != nil {
		log.Printf("peer added Event %v\n", peer.Name)
		runtime.EventsEmit(a.ctx, "peer-connected", discovery.ToPeerDTO(peer))
	}
}

func (a *App) OnPeerRemoved(peer discovery.Peer) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "peer-disconnected", discovery.ToPeerDTO(peer))
	}
}

func (a *App) SendFileToPeer(peerId string, filePath string) error {
	log.Printf("Enter App SendFileToPeer %v FilePath %v\n", peerId, filePath)

	selectedPeerSession, ok := peerSessions[peerId]
	if !ok {
		log.Printf("Peer %v not found in peer sessions %v\n", peerId, peerSessions)

		peerInfo := a.discovery.GetPeerById(peerId)
		if peerInfo == nil {
			return fmt.Errorf("peer %v not found", peerId)
		}

		log.Printf("Peer %v is selected by peer %v\n", peerId, peerInfo.Name)

		dialer := &transport.Dialer{}
		conn, err := dialer.Dial(*peerInfo, a.ctx, filePath)

		if err != nil {
			return fmt.Errorf("peer %v dial error: %v", peerId, err)
		}

		selectedPeerSession = session.NewPeerSession(a.ctx, conn)
		selectedPeerSession.Start(a.settings.DownloadPath)
		a.RegisterSession(selectedPeerSession)
	}

	fileName := filepath.Base(filePath)
	fileId := fmt.Sprintf("%v-%v", fileName, time.Now().UnixNano())

	a.transferRegistry.AddFileSending(peerId, fileId, fileName, transfer.InProgress, 0, 0)

	err := selectedPeerSession.SendLargeFile(a.ctx, filePath, 1024*1024, fileId)
	if err != nil {
		_ = selectedPeerSession.Stop()
		delete(peerSessions, peerId)

		//runtime.EventsEmit(a.ctx, "peer-disconnected", selectedPeerSession.)
		log.Printf("Peer disconnected %v\n During sending", peerId)
		a.discovery.RemovePeerById(peerId)

		err := addNewTransferFileHistory(selectedPeerSession.GetPeerInfo().UserName, filepath.Base(filePath), "SENT", "FAILED")

		if err != nil {
			log.Printf("Error adding file history: %v", err)
		}

		return fmt.Errorf("Error while creating connection during dialing %v\n", err)
	}

	log.Printf("Sending file to peer %v\n", peerId)

	err = addNewTransferFileHistory(selectedPeerSession.GetPeerInfo().UserName, filepath.Base(filePath), "SENT", "COMPLETED")

	if err != nil {
		log.Printf("Error adding file to peer %v\n", peerId)
	}

	return nil
}

func (a *App) PickFile() (string, error) {
	paths, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a file to send",
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no file selected")
	}
	return paths, nil
}

func (a *App) GetSettings() (*Settings, error) {
	settingsFile, err := getSettingsFilePath()

	if err != nil {
		return nil, err
	}

	var settings *Settings
	settings, err = readSettingsFile(*settingsFile)

	if err != nil {
		return nil, err
	}

	return settings, nil
}

func (a *App) SaveSettings(updatedSettings *Settings) error {
	log.Printf("Saving settings file %v\n", updatedSettings.IsPermissionRequiredToSendFiles)
	settings, err := a.GetSettings()
	if err != nil {
		return err
	}

	settings.DownloadPath = updatedSettings.DownloadPath
	settings.UserName = updatedSettings.UserName
	settings.IsPermissionRequiredToSendFiles = updatedSettings.IsPermissionRequiredToSendFiles

	data, err := json.MarshalIndent(updatedSettings, "", "  ")
	if err != nil {
		return err
	}
	settingsFilePath, err := getSettingsFilePath()

	if err != nil {
		return err
	}

	err = os.WriteFile(*settingsFilePath, data, 0600)

	if err != nil {
		log.Printf("Error while writing settings file %v\n", err)
		return err
	}

	return nil
}

func (a *App) PickDownloadFolder() (string, error) {
	dialog, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a folder to set as download directory",
	})

	if err != nil {
		return "", err
	}
	log.Printf("Dialog %v\n", dialog)
	return dialog, err
}

func (a *App) GetTransferHistories() []transferHistory {
	sentHistories, err := loadOrCreateTransferHistory()
	if err != nil {
		log.Printf("Error loading sent transfer history: %v\n", err)
		return nil
	}

	return sentHistories
}

func (a *App) ReceiveFilePermission(peerId string, fileName string, fileId string, allow bool) {
	log.Printf("Receive File Permission %v %v\n", peerId, fileName, fileId, allow)

	permissionResponse := protocol.Message{
		Type:    "file-permission",
		Name:    fileName,
		Id:      fileId,
		Allowed: allow,
	}

	a.mu.Lock()
	p, ok := a.pendingPermissions[fileId]
	delete(a.pendingPermissions, fileId)
	a.mu.Unlock()

	if !ok {
		return
	}
	err := p.session.Send(permissionResponse)
	log.Printf("Sending permission response %v\n", permissionResponse)
	if err != nil {
		log.Printf("Error sending permission response: %v\n", err)
		return
	}
}

func (a *App) ClearTransferHistory() {
	err := clearTransferHistories()

	if err != nil {
		log.Printf("Error clearing transfer history: %v\n", err)
	}
}

func (a *App) DisconnectPeer(peerId string) {
	peerSession, ok := peerSessions[peerId]
	if !ok {
		log.Printf("Peer %v not found in peer sessions %v\n", peerId, peerSessions)
		return
	}
	_ = peerSession.Stop()
	delete(peerSessions, peerId)
	log.Info("Peer disconnected %v", peerId)
	log.Printf("Peer List %v\n", a.ListPeers())
}
