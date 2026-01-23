package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/session"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	discovery *discovery.Discovery
	settings  *Settings
}

var sentHistories []transferHistory

func NewApp(d *discovery.Discovery) *App {
	return &App{
		discovery: d,
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	settings, err := loadOrCreateSettings()

	if err != nil {
		log.Fatal(err)
	}

	sentHistories, err := loadOrCreateSentTransferHistory()

	if err != nil {
		log.Printf("Error loading sent transfer history: %v", err)
	}

	a.settings = settings

	log.Printf("Loading sent transfer history%v\n", sentHistories)
}

func (a *App) Name(name string) string {
	log.Printf("Random event from go")
	runtime.EventsEmit(a.ctx, "randomEvent", name)
	return name
}

func (a *App) bindSession(p *session.PeerSession) {

	peer := p.GetPeerInfo()

	if a.ctx != nil {
		log.Printf("peer connected %v\n", peer)
		runtime.EventsEmit(a.ctx, "peer-connected", map[string]interface{}{
			"id":   peer.ID,
			"name": peer.Name,
			"port": peer.Port,
		})
	}

	p.OnFileReceived(func(name string) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "file-received", map[string]interface{}{
				"name": name,
				"peer": p.GetPeerInfo().Name,
			})
		}
	})

	p.OnDisconnected(func(peer discovery.Peer) {
		log.Printf("peer disconnected %v\n", peer.Name)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "disconnected", map[string]interface{}{
				"name": peer.Name,
				"id":   peer.ID,
				"port": peer.Port,
			})
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
		conn, err := dialer.Dial(*peerInfo, a.ctx)

		if err != nil {
			return fmt.Errorf("peer %v dial error: %v", peerId, err)
		}

		selectedPeerSession = session.NewPeerSession(conn)
		selectedPeerSession.Start(a.settings.DownloadPath)
		a.RegisterSession(selectedPeerSession)
	}

	err := selectedPeerSession.SendLargeFile(a.ctx, filePath, 1024*1024)
	if err != nil {
		log.Printf("Error while creating connection during dialing %v\n", err)
		return fmt.Errorf("Error while creating connection during dialing %v\n", err)
	}

	log.Printf("Sending file to peer %v\n", peerId)

	err = addNewSentFileHistory(selectedPeerSession.GetPeerInfo().Name, filePath, "COMPLETED")

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
	log.Printf("Saving settings file %v\n", *updatedSettings)
	settings, err := a.GetSettings()
	if err != nil {
		return err
	}

	settings.DownloadPath = updatedSettings.DownloadPath
	settings.UserName = updatedSettings.UserName

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
	sentHistories, err := loadOrCreateSentTransferHistory()
	if err != nil {
		log.Printf("Error loading sent transfer history: %v\n", err)
		return nil
	}
	
	return sentHistories
}
