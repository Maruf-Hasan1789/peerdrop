package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
	transport2 "github.com/Maruf-Hasan1789/peerdrop/internal/transport"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type chunkedFile struct {
	file           *os.File
	fileMeta       protocol.FileMeta
	totalChunks    int64
	chunkSize      int
	receivedCount  int64
	receivedChunks map[int]bool
	status         string
	lastEmit       time.Time
	rootId         string
}

var incomingFiles = make(map[string]*chunkedFile)

type PeerSession struct {
	ID     string
	ctx    context.Context
	cancel context.CancelFunc
	conn   transport.Connection
	peer   discovery.Peer
	config transport2.SessionConfig

	done chan struct{}

	//session metadata
	connectedAt time.Time
	lastSeen    time.Time

	//event listeners
	onDisconnected        func(peer discovery.Peer)
	onError               func(err error)
	fileReceivedListeners []func(peerId string, rootId string, rootName string)

	onFileOffer                func(peerId string, transferId string, rootEntry []*protocol.RootEntry, totalSize int64)
	onTransferStart            func(peerId string, transferId string, rootId string, rootName string)
	onTransferCompletion       func(peerId string, transferId string, rootId string, rootName string)
	onTransferProgress         func(peerId string, transferId, rootId string, rootName string, progress float64)
	onTransferError            func(peerId string, transferId string, rootId string, rootName string, err error)
	onTransferPermissionDenied func(peerId string, transferId string, rootEntries []*protocol.RootEntry)

	//message handlers map
	handlers               map[protocol.MessageType]func(msg protocol.Message, downloadPath string)
	fileSendingPermissions map[string]chan *protocol.Control
	mu                     sync.Mutex
	activeTransfers        map[string]context.CancelFunc

	RootPaths             map[string]string
	rootEntries           map[string]*protocol.RootEntry
	rootReceivingProgress map[string]*RootProgress
	rootSendingProgress   map[string]*RootProgress
}

type RootProgress struct {
	RootID           string
	TotalBytes       int64
	TransferredBytes int64
	LastEmit         time.Time
	mu               sync.Mutex
}

func NewPeerSession(ctx context.Context, conn transport.Connection, config transport2.SessionConfig) *PeerSession {
	ctx, cancel := context.WithCancel(ctx)
	p := &PeerSession{
		ID:                     uuid.NewString(),
		ctx:                    ctx,
		cancel:                 cancel,
		conn:                   conn,
		peer:                   conn.PeerInfo(),
		config:                 config,
		done:                   make(chan struct{}),
		connectedAt:            time.Now(),
		handlers:               make(map[protocol.MessageType]func(msg protocol.Message, downloadPath string)),
		lastSeen:               time.Now(),
		fileSendingPermissions: make(map[string]chan *protocol.Control),
		rootPaths:              make(map[string]string),
		rootEntries:            make(map[string]*protocol.RootEntry),
		rootReceivingProgress:  make(map[string]*RootProgress),
		rootSendingProgress:    make(map[string]*RootProgress),
	}

	p.handlers["CONTROL"] = p.handleControl
	p.handlers["CHUNK"] = p.handleChunk
	p.handlers["HANDSHAKE"] = p.handleHandshake
	return p
}

func (p *PeerSession) Start(downloadPath string) {
	go p.readLoop(downloadPath)
}

func (p *PeerSession) Stop() error {
	p.cancel()
	err := p.conn.Close()
	<-p.done
	return err
}

func (p *PeerSession) Done() <-chan struct{} {
	return p.done
}

func (p *PeerSession) OnDisconnected(fn func(peer discovery.Peer)) {
	p.onDisconnected = fn
}

func (p *PeerSession) OnError(fn func(err error)) {
	p.onError = fn
}

func (p *PeerSession) OnFileReceived(fn func(peerId string, rootId string, rootName string)) {
	p.fileReceivedListeners = append(p.fileReceivedListeners, fn)
}

func (p *PeerSession) OnFileOffer(fn func(peerId string, transferId string, rootEntries []*protocol.RootEntry, totalSize int64)) {
	p.onFileOffer = fn
}

func (p *PeerSession) OnTransferStart(fn func(peerId string, transferId string, rootId string, rootName string)) {
	p.onTransferStart = fn
}

func (p *PeerSession) OnTransferCompletion(fn func(peerId string, transferId string, rootId string, rootName string)) {
	p.onTransferCompletion = fn
}

func (p *PeerSession) OnTransferProgress(fn func(peerId string, transferId, rootId string, rootName string, progress float64)) {
	p.onTransferProgress = fn
}
func (p *PeerSession) OnTransferError(fn func(peerId string, transferId string, rootId string, rootName string, err error)) {
	p.onTransferError = fn
}
func (p *PeerSession) OnTransferPermissionDenied(fn func(peerId string, transferId string, rootEntries []*protocol.RootEntry)) {
	p.onTransferPermissionDenied = fn
}
func (p *PeerSession) readLoop(downloadPath string) {
	defer close(p.done)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			data, err := p.conn.Receive()

			if err != nil {
				if p.ctx.Err() == nil {
					log.Printf("Peer disconnected during reading %v\n", err)
					p.handleDisconnect(err)
					return
				}

				log.Printf("Peer session stopped due to context cancellation\n")
			}

			p.handleMessage(data, downloadPath)
		}
	}
}

func (p *PeerSession) handleMessage(data []byte, downloadPath string) {
	var msg protocol.Message

	if err := json.Unmarshal(data, &msg); err != nil {
		if p.onError != nil {
			p.onError(fmt.Errorf("invalid message: %w", err))
		}
		return
	}

	handler, ok := p.handlers[msg.Type]

	if ok {
		handler(msg, downloadPath)
	} else {
		if p.onError != nil {
			p.onError(fmt.Errorf("unknown message type: %s", msg.Type))
		}
	}
}

func (p *PeerSession) handleControl(msg protocol.Message, downloadPath string) {

	log.Printf("Handling Control %v\n", msg)

	if msg.Control.Action == protocol.ActionHandshakeAck {
		p.mu.Lock()

		log.Printf("Message ID %v", msg.ID)

		ch, ok := p.fileSendingPermissions[msg.ID]
		p.mu.Unlock()

		if !ok {
			log.Printf("File Sending Permissions channel not found for %v\n", msg.ID)
		}

		delete(p.fileSendingPermissions, msg.ID)

		if ch == nil {
			log.Printf("Permission channel is missing")
		}

		ch <- msg.Control
	}
}

func (p *PeerSession) Send(msg protocol.Message) error {
	payload, err := json.Marshal(msg)

	if err != nil {
		log.Printf("Error while sending msg = %v %v\n", msg, err)
		return err
	}

	return p.conn.Send(payload)
}

type FileResult struct {
	FileId   string
	FileName string
	Error    error
}

func (p *PeerSession) SendToPeer(transferId string, rootEntries []*protocol.RootEntry) error {
	permCh := p.waitForPermission(transferId)

	for _, rootEntry := range rootEntries {
		for _, file := range rootEntry.Files {
			log.Printf("Root Entry Name = %v FilePath %v\n", rootEntry.Name, file.Path)
		}
	}

	err := p.SendHandshakesForFiles(rootEntries, transferId)

	if err != nil {
		log.Printf("Error sending handshake for files: %v\n", err)
		return err
	}

	ctx := p.ctx
	ctx, _ = context.WithCancel(ctx)

	select {
	case control := <-permCh:
		if control.Mode == protocol.PermissionNone {
			log.Printf("Permission is denied\n")

			if p.onTransferPermissionDenied != nil {
				p.onTransferPermissionDenied(p.peer.ID, transferId, rootEntries)
			}

			ctx.Done()
		} else if control.Mode == protocol.PermissionAll {

			for _, rootEntry := range rootEntries {
				//	wg.Add(1)
				rootPath := p.rootPaths[rootEntry.ID]
				go p.SendRootEntry(transferId, rootEntry, rootPath)
			}
		} else {
			log.Printf("Partial Permission is allowed\n")
			//fileResultCh := make(chan FileResult, len(rootEntries))
			//wg := sync.WaitGroup{}
			//go showFileResults(fileResultCh)
			allowedRootID := make(map[string]bool)

			for _, file := range control.Files {
				if file.Allowed {
					allowedRootID[file.FileID] = true
				} else {
					allowedRootID[file.FileID] = false
				}
			}

			for _, rootEntry := range rootEntries {

				isAllowed, ok := allowedRootID[rootEntry.ID]
				if !ok || !isAllowed {
					continue
				}

				rootPath := p.rootPaths[rootEntry.ID]
				go p.SendRootEntry(transferId, rootEntry, rootPath)
			}

			//wg.Wait()
			//close(fileResultCh)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (p *PeerSession) SendRootEntry(transferId string, rootEntry *protocol.RootEntry, rootPath string) {
	for _, file := range rootEntry.Files {
		err := p.Sendfile(transferId, file, rootPath, rootEntry)
		if err != nil {
			log.Printf("Error sending file %v error : %v\n", file.Path, err)
			return
		}
	}
}

func (p *PeerSession) SendHandshakesForFiles(rootEntries []*protocol.RootEntry, transferId string) error {

	fileOffer := protocol.Message{
		Version:   protocol.ProtocolVersion,
		Type:      protocol.TypeHandshake,
		ID:        transferId,
		Handshake: &protocol.Handshake{},
	}

	var totalSize int64 = 0

	for _, rootEntry := range rootEntries {
		totalSize += rootEntry.Size
	}

	fileOffer.Handshake.Roots = rootEntries
	fileOffer.Handshake.TotalSize = totalSize
	fileOffer.Handshake.ChunkSize = p.config.ChunkSize

	err := p.Send(fileOffer)

	return err
}

func (p *PeerSession) Sendfile(transferId string, fileMeta protocol.FileMeta, rootPath string, rootEntry *protocol.RootEntry) error {
	peerId := p.peer.ID
	filePath := rootPath

	if rootEntry.Type == protocol.EntryTypeDirectory {
		filePath = filepath.Join(rootPath, fileMeta.Path)
	}

	log.Printf("File Path of rootEntry Type = %v rootPath = %v FileMetaPath = %v\n filePath = %v\n", rootEntry.Type, rootPath, fileMeta.Path, filePath)
	f, err := os.Open(filePath)

	if err != nil {
		log.Printf("Error opening file %v: %v", fileMeta.Path, err)

		if p.onTransferError != nil {
			p.onTransferError(peerId, transferId, rootEntry.ID, rootEntry.Name, err)
		}

		return err
	}

	defer f.Close()

	fi, _ := f.Stat()

	chunkSize := p.config.ChunkSize

	totalChunks := int((fi.Size() + int64(chunkSize) - 1) / int64(chunkSize))

	log.Printf("total chunks %v\n", totalChunks)

	buf := make([]byte, chunkSize)

	rootId := rootEntry.ID

	rootSendProgress, ok := p.rootSendingProgress[rootId]

	if !ok {
		rootSendProgress = &RootProgress{
			RootID:           rootId,
			TotalBytes:       rootEntry.Size,
			TransferredBytes: 0,
			LastEmit:         time.Now(),
		}

		p.rootSendingProgress[rootId] = rootSendProgress

		if p.onTransferStart != nil {
			p.onTransferStart(peerId, transferId, rootId, rootEntry.Name)
		}
	}

	for i := 0; i < totalChunks; i++ {
		n, err := f.Read(buf)

		if err != nil && err != io.EOF {
			p.onTransferError(peerId, transferId, rootId, rootEntry.Name, err)
			return err
		}

		hash := sha256.New()
		hash.Write(buf[:n])
		checkSum := hash.Sum(nil)

		chunk := &protocol.Chunk{
			FileID:   fileMeta.ID,
			RootId:   rootEntry.ID,
			Index:    i,
			Total:    totalChunks,
			Offset:   int64(i * chunkSize),
			Size:     fileMeta.Size,
			Data:     buf[:n],
			CheckSum: hex.EncodeToString(checkSum),
		}

		msg := protocol.Message{
			Version: protocol.ProtocolVersion,
			Type:    protocol.TypeChunk,
			ID:      transferId,
			Chunk:   chunk,
		}

		if err := p.Send(msg); err != nil {
			log.Printf("Here on line 403\n")
			p.onTransferError(peerId, transferId, rootId, rootEntry.Name, err)
			return err
		}

		rootSendProgress.mu.Lock()
		rootSendProgress.TransferredBytes += int64(len(msg.Chunk.Data))
		rootSendProgress.mu.Unlock()

		if time.Since(rootSendProgress.LastEmit) > time.Second {
			rootSendProgress.LastEmit = time.Now()
			if p.onTransferProgress != nil {
				progress := float64(rootSendProgress.TransferredBytes) / float64(rootSendProgress.TotalBytes)
				p.onTransferProgress(peerId, transferId, rootId, rootEntry.Name, progress)
			}
		}
	}

	if rootSendProgress.TotalBytes == rootSendProgress.TransferredBytes {
		if p.onTransferCompletion != nil {
			p.onTransferCompletion(peerId, transferId, rootId, rootEntry.Name)
		}
	}

	return nil
}

func openFile(originalFileName string) (*os.File, error) {
	//log.Printf("Opening file %v\n", originalFileName)
	dir := filepath.Dir(originalFileName)
	ext := filepath.Ext(originalFileName)
	base := filepath.Base(originalFileName[:len(originalFileName)-len(ext)])

	for i := 0; ; i++ {
		name := base + ext
		if i > 0 {
			name = fmt.Sprintf("%s(%d)%s", base, i, ext)
		}

		newPath := filepath.Join(dir, name)
		//log.Printf("Opening file %v\n", newPath)
		file, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)

		if err == nil {
			log.Printf("Opened file %v\n", newPath)
			return file, nil
		}

		log.Printf("Here error in opening file %v\n", err)
	}
}

func (p *PeerSession) handleChunk(msg protocol.Message, downloadPath string) {
	incomingFileId := getIncomingFileId(msg.ID, msg.Chunk.RootId, msg.Chunk.FileID)
	f, ok := incomingFiles[incomingFileId]

	if !ok {
		log.Printf("File was not mentioned in handshake\n")
		return
	}

	fileName := f.fileMeta.Path

	if f.file == nil {

		rootEntry, ok := p.rootEntries[f.rootId]
		if !ok {
			log.Printf("No root entry for file %v\n", f.fileMeta.Path)
			return
		}

		var filePath string
		if rootEntry.Type == protocol.EntryTypeDirectory {
			filePath = filepath.Join(downloadPath, rootEntry.Name, fileName)

			err := os.MkdirAll(filepath.Dir(filePath), 0755)
			if err != nil {
				log.Printf("Error creating directory: %v\n", err)
				return
			}
		} else {
			filePath = filepath.Join(downloadPath, fileName)
		}

		file, err := openFile(filePath)
		if err != nil {
			log.Printf("Error opening file %v\n", err)
			return
		}

		f.file = file

		rProgress, ok := p.rootReceivingProgress[rootEntry.ID]

		if !ok {
			log.Printf("Here rProgress not present")
			rProgress = &RootProgress{
				RootID:           rootEntry.ID,
				TotalBytes:       rootEntry.Size,
				TransferredBytes: 0,
				LastEmit:         time.Now(),
			}
			p.rootReceivingProgress[rootEntry.ID] = rProgress

			runtime.EventsEmit(p.ctx, "receiving-started", map[string]string{
				"id":            f.rootId,
				"file":          p.rootEntries[f.rootId].Name,
				"totalChunks":   strconv.FormatInt(rProgress.TotalBytes, 10),
				"totalReceived": "0",
			})
		}
	}

	hash := sha256.New()
	hash.Write(msg.Chunk.Data)

	finalHash := hash.Sum(nil)
	receivedChecksum := hex.EncodeToString(finalHash)

	if receivedChecksum == msg.Chunk.CheckSum && f.receivedChunks[msg.Chunk.Index] == false {
		offset := msg.Chunk.Offset
		_, err := f.file.WriteAt(msg.Chunk.Data, offset)

		if err != nil {
			log.Printf("Error writing to file %v: %v", f.fileMeta.Path, err)
			return
		}
		f.receivedChunks[msg.Chunk.Index] = true
		f.receivedCount++
	} else {
		log.Printf("Received CheckSum %v is not equal to calculated Checksum %v\n", receivedChecksum, msg.Chunk.CheckSum)
		return
	}

	rProgress := p.rootReceivingProgress[f.rootId]

	rProgress.mu.Lock()
	rProgress.TransferredBytes += int64(len(msg.Chunk.Data))
	rProgress.mu.Unlock()

	if time.Since(rProgress.LastEmit) >= time.Second {
		runtime.EventsEmit(p.ctx, "receiving-progress", map[string]string{
			"id":            rProgress.RootID,
			"file":          p.rootEntries[rProgress.RootID].Name,
			"totalBytes":    strconv.FormatInt(rProgress.TotalBytes, 10),
			"totalReceived": strconv.FormatInt(rProgress.TransferredBytes, 10),
		})
		rProgress.LastEmit = time.Now()
	}

	//check if all chunks received

	complete := false

	if f.receivedCount == f.totalChunks {
		complete = true
	}

	if complete {
		log.Printf("File: %v is received successfully\n", fileName)
		fileInfo, err := f.file.Stat()
		if err != nil {
			log.Printf("Error getting file size %v\n", err)
		}

		err = f.file.Truncate(fileInfo.Size())

		if err != nil {
			log.Printf("Error truncating file %v: %v", f.fileMeta.Path, err)
			return
		}

		err = f.file.Sync()

		if err != nil {
			log.Printf("Error syncing file %v: %v", f.fileMeta.Path, err)
			return
		}

		err = f.file.Sync()
		if err != nil {
			log.Printf("Error syncing file %v: %v", f.fileMeta.Path, err)
			return
		}

		_ = f.file.Close()

		log.Printf("Large file received %v\n", fileName)

		if rProgress.TotalBytes == rProgress.TransferredBytes {
			for _, fn := range p.fileReceivedListeners {
				fn(p.peer.ID, rProgress.RootID, p.rootEntries[rProgress.RootID].Name)
			}
		}

		delete(incomingFiles, incomingFileId)
	}
}

func (p *PeerSession) GetPeerInfo() discovery.Peer {
	return p.peer
}

func (p *PeerSession) handleDisconnect(err error) {
	log.Printf("Peer disconnected during reading %v %v\n", p.peer.Name, err)

	if p.onDisconnected != nil {
		log.Printf("Peer On Disconnected Provided: %v\n", p.peer.Name)
		p.onDisconnected(p.peer)
	}

	if p.onError != nil && err != io.EOF {
		p.onError(err)
	}
}

func (p *PeerSession) handleHandshake(msg protocol.Message, path string) {
	log.Printf("Handling Handshake %v\n", msg)

	for _, rootEntry := range msg.Handshake.Roots {
		p.rootEntries[rootEntry.ID] = rootEntry

		for _, file := range rootEntry.Files {
			incomingFileId := getIncomingFileId(msg.ID, rootEntry.ID, file.ID)
			incomingFiles[incomingFileId] = &chunkedFile{
				totalChunks:    (file.Size + int64(msg.Handshake.ChunkSize-1)) / int64(msg.Handshake.ChunkSize),
				chunkSize:      msg.Handshake.ChunkSize,
				fileMeta:       file,
				receivedCount:  0,
				receivedChunks: make(map[int]bool),
				status:         "Initiated",
				lastEmit:       time.Now(),
				rootId:         rootEntry.ID,
			}
		}
	}

	if p.onFileOffer != nil {
		p.onFileOffer(p.peer.ID, msg.ID, msg.Handshake.Roots, msg.Handshake.TotalSize)
	}
}

func getIncomingFileId(transferId string, rootId string, fileId string) string {
	return fmt.Sprintf("%v|%v|%v", transferId, rootId, fileId)
}

func (p *PeerSession) waitForPermission(transferId string) <-chan *protocol.Control {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan *protocol.Control)
	p.fileSendingPermissions[transferId] = ch

	log.Printf("Waiting for permission %v %v\n", transferId, p.fileSendingPermissions[transferId])

	return ch
}

func (p *PeerSession) getRootEntriesFromFilePaths(paths []string) []*protocol.RootEntry {
	var rootEntries []*protocol.RootEntry
	for _, path := range paths {
		rootEntry, err := getRootEntryFromPath(path)
		if err != nil {
			log.Printf("Error getting file info: %v\n", err)
			continue
		}

		p.rootPaths[rootEntry.ID] = path
		rootEntries = append(rootEntries, rootEntry)
	}

	for _, rootEntry := range rootEntries {
		log.Printf("Root Name = %v root Size = %v\n", rootEntry.Name, rootEntry.Size)
	}

	return rootEntries
}

func (p *PeerSession) CancelTransfer(transferId string, rootId string) {

}

func getRootEntryFromPath(path string) (*protocol.RootEntry, error) {
	fileInfo, err := os.Stat(path)

	if err != nil {
		log.Printf("Error stating file %v: %v", path, err)
		return nil, err
	}

	rootId := uuid.NewString()

	if !fileInfo.IsDir() {
		fileName := filepath.Base(path)
		fileId := fmt.Sprintf("%v_%v", filepath.Base(path), time.Now().UnixNano())
		fileSize := fileInfo.Size()

		return &protocol.RootEntry{
			ID:   rootId,
			Name: fileName,
			Type: protocol.EntryTypeFile,
			Size: fileSize,
			Files: []protocol.FileMeta{{
				ID:   fileId,
				Path: fileName,
				Size: fileSize},
			},
		}, nil
	}

	rootEntry := &protocol.RootEntry{
		ID:    rootId,
		Name:  filepath.Base(path),
		Type:  protocol.EntryTypeDirectory,
		Files: []protocol.FileMeta{},
	}

	root := path
	var totalSize int64 = 0

	err = filepath.Walk(root, func(dirPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(root, dirPath)

		if err != nil {
			return err
		}

		rootEntry.Files = append(rootEntry.Files, protocol.FileMeta{
			ID:   fmt.Sprintf("%v_%v", filepath.Base(relativePath), time.Now().UnixNano()),
			Path: relativePath,
			Size: info.Size(),
		})
		totalSize += info.Size()

		return nil
	})

	rootEntry.Size = totalSize

	return rootEntry, err
}
