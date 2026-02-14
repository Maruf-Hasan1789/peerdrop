package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/google/uuid"
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

type FileReceivePermissionResponse struct {
	Mode  protocol.PermissionMode `json:"mode"`
	Files map[string]bool         `json:"files"`
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
	log.Printf("Here in bind session\n")
	p.OnFileReceived(func(peerId string, rootId string, rootName string) {
		log.Printf("File received: in Bind Session %s", rootName)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "file-received", map[string]interface{}{
				"id":       rootId,
				"fileName": rootName,
				"peer":     p.GetPeerInfo().UserName,
			})
		}
	})

	p.OnDisconnected(func(peer discovery.Peer) {
		log.Info("peer disconnected %v %v\n", peer.Name, peer.UserName)
		if a.ctx != nil {
			//a.discovery.RemovePeerById(peer.ID)

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

	p.OnTransferStart(func(peerId string, transferId string, rootId string, rootName string) {
		//log.Printf("Transfer Started \n")
		//need to update the transferred bytes and totalbytes in future
		a.transferRegistry.AddTransferredFile(peerId, transferId, rootId, rootName, transfer.InProgress, 0, 0, transfer.Outgoing)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "transfer-start", map[string]string{
				"id":         rootId,
				"peerId":     peerId,
				"fileName":   rootName,
				"transferId": transferId + rootId,
			})
		}
	})

	p.OnTransferCompletion(func(peerId string, transferId string, rootId string, rootName string) {
		log.Printf("Here on transfer completion")

		a.transferRegistry.RemoveFileRegistryUponCompletion(peerId, rootId)
		log.Printf("Sending file to peer %v\n", peerId)

		err := addNewTransferFileHistory(p.GetPeerInfo().UserName, rootName, "SENT", "COMPLETED", transferId)

		if err != nil {
			log.Printf("Error adding file to peer %v\n", peerId)
		}

		runtime.EventsEmit(a.ctx, "transfer-complete", map[string]string{
			"id":         rootId,
			"peerId":     peerId,
			"file":       rootName,
			"transferId": transferId + rootId,
		})
	})

	p.OnTransferProgress(func(peerId string, transferId string, rootId string, rootName string, progress float64) {
		log.Printf("On Transfer Progress \n")
		runtime.EventsEmit(a.ctx, "transfer-progress", map[string]string{
			"id":         rootId,
			"peerId":     peerId,
			"file":       rootName,
			"progress":   fmt.Sprintf("%.2f", progress),
			"transferId": transferId + rootId,
		})
	})

	p.OnTransferError(func(peerId string, transferId string, rootId string, rootName string, err error) {
		log.Printf("Here on event handler on Transfer Error\n")

		log.Printf("Emitting Events\n")

		transferRegistry := a.transferRegistry.GetAllTransfersByPeerId(peerId)

		log.Printf("On Transfer Error transfer registry len : %v\n", len(transferRegistry))

		for _, t := range transferRegistry {
			log.Printf("Transfer Registry : %v\n", t.RootName)
			if t.PeerId == peerId && (t.Status == transfer.InProgress || t.Status == transfer.Pending) &&
				t.Direction == transfer.Outgoing {
				err := addNewTransferFileHistory(p.GetPeerInfo().UserName, t.RootName, "SENT", "FAILED", t.RootID)
				if err != nil {
					log.Printf("Error adding file history: %v", err)
				}

			}
		}

		runtime.EventsEmit(a.ctx, "transfer-failed", map[string]string{
			"id":         rootId,
			"peerId":     peerId,
			"file":       rootName,
			"transferId": transferId + rootId,
		})
	})

	p.OnTransferPermissionDenied(func(peerId string, transferId string, rootEntries []*protocol.RootEntry) {
		log.Printf("Here on permission denied")
		rootNames := make([]string, 0)

		for _, entry := range rootEntries {
			rootNames = append(rootNames, entry.Name)
		}
		log.Printf("Here rootNames %v\n", rootNames)

		runtime.EventsEmit(a.ctx, "transfer-permission-denied", map[string]interface{}{
			"peerId":     peerId,
			"transferId": transferId,
			"files":      rootNames,
		})
	})
}

var peerSessions = make(map[string]*session.PeerSession)

func (a *App) RegisterSession(peerSession *session.PeerSession) {
	log.Printf("Here is register session\n")
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

func (a *App) SendFileToPeer(peerId string, filePaths []string) error {
	log.Printf("Here in send file to peer\n")
	log.Printf("Enter App SendFileToPeer %v FilePath %v\n", peerId, filePaths)

	selectedPeerSession, ok := peerSessions[peerId]
	if !ok {
		log.Printf("Peer %v not found in peer sessions %v\n", peerId, peerSessions)

		peerInfo := a.discovery.GetPeerById(peerId)
		if peerInfo == nil {
			return fmt.Errorf("peer %v not found", peerId)
		}

		log.Printf("Peer %v is selected by peer %v\n", peerId, peerInfo.Name)

		dialer := &transport.Dialer{}
		conn, err := dialer.Dial(*peerInfo, a.ctx)

		if err != nil {
			return fmt.Errorf("peer %v dial error: %v", peerId, err)
		}

		selectedPeerSession = session.NewPeerSession(a.ctx, conn)
		selectedPeerSession.Start(a.settings.DownloadPath)
		log.Printf("Here before registering session\n")
		a.RegisterSession(selectedPeerSession)
	}

	transferId := uuid.NewString()

	err := a.transferFileToPeer(selectedPeerSession, filePaths, transferId)

	return err
}

func (a *App) transferFileToPeer(peerSession *session.PeerSession, filePaths []string, transferId string) error {

	peerId := peerSession.GetPeerInfo().ID
	startingTime := time.Now()
	err := peerSession.SendToPeer(transferId, filePaths)

	if err != nil {
		_ = peerSession.Stop()
		delete(peerSessions, peerId)

		//runtime.EventsEmit(a.ctx, "peer-disconnected", selectedPeerSession.)
		log.Printf("Peer disconnected %v\n During sending", peerId)
		a.discovery.RemovePeerById(peerId)
		return err
	}

	log.Printf("Total taken Time %v\n", time.Since(startingTime).Seconds())

	log.Printf("Sent successfully")

	return nil
}

func (a *App) PickFiles() ([]string, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select files to send",
		Filters: []runtime.FileFilter{
			{DisplayName: "File", Pattern: "*.*"},
		},
	})

	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no file selected")
	}

	return paths, nil
}

func (a *App) PickFolder() ([]string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a folders to send",
	})

	if err != nil {
		log.Printf("Error opening folders: %v\n", err)
		return nil, err
	}

	if len(path) == 0 {
		return nil, fmt.Errorf("no folders selected")
	}

	return []string{path}, nil
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

func (a *App) ReceiveFilePermission(peerId string, transferId string, permResp FileReceivePermissionResponse) {

	log.Printf("Receive File Permission %v %v %v \n", peerId, permResp, a.settings.DownloadPath)

	filePermissionControl := &protocol.Control{
		Action:  protocol.ActionHandshakeAck,
		Details: fmt.Sprintf("Files permission"),
	}

	if permResp.Mode == protocol.PermissionAll {
		filePermissionControl.Mode = protocol.PermissionAll
		for rootId, _ := range permResp.Files {
			a.transferRegistry.UpdateTransferRegistryStatusByRootId(rootId, transfer.InProgress)
		}

	} else if permResp.Mode == protocol.PermissionNone {
		filePermissionControl.Mode = protocol.PermissionNone
		for rootId, _ := range permResp.Files {
			a.transferRegistry.UpdateTransferRegistryStatusByRootId(rootId, transfer.Rejected)
		}
	} else {
		filePermissionControl.Mode = protocol.PermissionPartial
		var controls []protocol.FileControl

		for rootId, isAllowed := range permResp.Files {
			var transferStatus transfer.Status

			if isAllowed == true {
				transferStatus = transfer.InProgress
			} else {
				transferStatus = transfer.Rejected
			}

			a.transferRegistry.UpdateTransferRegistryStatusByRootId(rootId, transferStatus)

			controls = append(controls, protocol.FileControl{
				FileID:  rootId,
				Allowed: isAllowed,
			})
		}

		filePermissionControl.Files = controls
	}

	permissionResponse := protocol.Message{
		Version: protocol.ProtocolVersion,
		Type:    protocol.TypeControl,
		ID:      transferId,
		Control: filePermissionControl,
	}

	log.Printf("Transfer ID %v\n", transferId)
	a.mu.Lock()
	p, ok := a.pendingPermissions[transferId]
	delete(a.pendingPermissions, transferId)
	a.mu.Unlock()

	if !ok {
		log.Printf("Transfer Id %v not found\n", transferId)
		return
	}
	log.Printf("Permission Response %v\n", permissionResponse)

	err := p.session.Send(permissionResponse)
	log.Printf("Sending permission response %v\n", permissionResponse)
	if err != nil {
		log.Printf("Error sending permission response: %v\n", err)
		return
	}
}

// just logging function
// to identify if transfer registry is updated properly or not
// will remove later on
func (a *App) showTransferRegistryStatus(incomingFilePermissions map[string]bool) {
	for rootId, isAllowed := range incomingFilePermissions {
		registry := a.transferRegistry.GetTransferRegistryByRootId(rootId)
		log.Printf("Transfer Registry Status for %v: %v %v\n", rootId, isAllowed, (*registry).Status)
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
