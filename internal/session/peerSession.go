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

	done chan struct{}

	//session metadata
	connectedAt time.Time
	lastSeen    time.Time

	//event listeners
	onDisconnected        func(peer discovery.Peer)
	onError               func(err error)
	fileReceivedListeners []func(peerId string, rootId string, rootName string)
	onFileOffer           func(peerId string, transferId string, rootEntry []*protocol.RootEntry, totalSize int64)

	//message handlers map
	handlers               map[protocol.MessageType]func(msg protocol.Message, downloadPath string)
	fileSendingPermissions map[string]chan bool
	mu                     sync.Mutex
	activeTransfers        map[string]context.CancelFunc

	rootPaths             map[string]string
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

func NewPeerSession(ctx context.Context, conn transport.Connection) *PeerSession {
	ctx, cancel := context.WithCancel(ctx)
	p := &PeerSession{
		ID:                     uuid.NewString(),
		ctx:                    ctx,
		cancel:                 cancel,
		conn:                   conn,
		peer:                   conn.PeerInfo(),
		done:                   make(chan struct{}),
		connectedAt:            time.Now(),
		handlers:               make(map[protocol.MessageType]func(msg protocol.Message, downloadPath string)),
		lastSeen:               time.Now(),
		fileSendingPermissions: make(map[string]chan bool),
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

		ch <- msg.Control.AllowAll
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

func (p *PeerSession) SendToPeer(transferId string, filePaths []string) error {
	permCh := p.waitForPermission(transferId)
	rootEntries := p.getRootEntriesFromFilePaths(filePaths)

	for _, rootEntry := range rootEntries {
		for _, file := range rootEntry.Files {
			log.Printf("Root Entry Name = %v FilePath %v\n", rootEntry.Name, file.Path)
		}
	}

	err := p.SendHandshakesForFiles(rootEntries, 1024*1024, transferId)

	if err != nil {
		log.Printf("Error sending handshake for files: %v\n", err)
		return err
	}

	ctx := p.ctx
	ctx, _ = context.WithCancel(ctx)

	select {
	case isAllowed := <-permCh:
		if !isAllowed {
			log.Printf("Permission is denied\n")
		} else {
			log.Printf("Permission is allowed\n")
			//fileResultCh := make(chan FileResult, len(rootEntries))
			//wg := sync.WaitGroup{}
			//go showFileResults(fileResultCh)
			for _, rootEntry := range rootEntries {
				//	wg.Add(1)
				rootPath := p.rootPaths[rootEntry.ID]
				go p.SendRootEntry(transferId, rootEntry, rootPath, 1024*1024)
			}

			//wg.Wait()
			//close(fileResultCh)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil

}

func (p *PeerSession) SendRootEntry(transferId string, rootEntry *protocol.RootEntry, rootPath string, chunkSize int) {
	for _, file := range rootEntry.Files {
		err := p.Sendfile(transferId, file, rootPath, rootEntry, chunkSize)
		if err != nil {
			log.Printf("Error sending file %v error : %v\n", file.Path, err)
			return
		}
	}
}

func (p *PeerSession) SendHandshakesForFiles(rootEntries []*protocol.RootEntry, chunkSize int, transferId string) error {

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
	fileOffer.Handshake.ChunkSize = chunkSize

	err := p.Send(fileOffer)

	return err
}

func (p *PeerSession) Sendfile(transferId string, fileMeta protocol.FileMeta, rootPath string, rootEntry *protocol.RootEntry, chunkSize int) error {
	ctx := p.ctx
	peerId := p.peer.ID
	filePath := rootPath

	if rootEntry.Type == protocol.EntryTypeDirectory {
		filePath = filepath.Join(rootPath, fileMeta.Path)
	}

	log.Printf("File Path of rootEntry Type = %v rootPath = %v FileMetaPath = %v\n filePath = %v\n", rootEntry.Type, rootPath, fileMeta.Path, filePath)
	f, err := os.Open(filePath)

	if err != nil {
		log.Printf("Error opening file %v: %v", fileMeta.Path, err)
		emitTransferFailedEvent(p.ctx, peerId, rootEntry.ID, rootEntry.Name, transferId)
		/*fileResultCh <- FileResult{
			FileId:   fileInfo.FileId,
			FileName: fileInfo.FileName,
			Error:    err,
		}
		*/
		return err
	}

	defer f.Close()

	fi, _ := f.Stat()

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

		runtime.EventsEmit(ctx, "transfer-start", map[string]string{
			"id":         rootId,
			"peerId":     peerId,
			"fileName":   rootEntry.Name,
			"transferId": transferId + rootId,
		})
	}

	for i := 0; i < totalChunks; i++ {
		n, err := f.Read(buf)

		if err != nil && err != io.EOF {
			emitTransferFailedEvent(ctx, peerId, fileMeta.ID, fileMeta.Path, transferId)
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
			emitTransferFailedEvent(ctx, peerId, rootId, rootEntry.Name, transferId)
			return err
		}

		rootSendProgress.mu.Lock()
		rootSendProgress.TransferredBytes += int64(len(msg.Chunk.Data))
		rootSendProgress.mu.Unlock()

		if time.Since(rootSendProgress.LastEmit) > time.Second {
			runtime.EventsEmit(ctx, "transfer-progress", map[string]string{
				"id":            rootId,
				"peerId":        peerId,
				"file":          rootEntry.Name,
				"totalBytes":    strconv.FormatInt(rootSendProgress.TotalBytes, 10),
				"totalReceived": strconv.FormatInt(rootSendProgress.TransferredBytes, 10),
				"transferId":    transferId + rootId,
			})
			rootSendProgress.LastEmit = time.Now()
		}
	}

	if rootSendProgress.TotalBytes == rootSendProgress.TransferredBytes {
		runtime.EventsEmit(ctx, "transfer-complete", map[string]string{
			"id":         rootId,
			"peerId":     peerId,
			"file":       rootEntry.Name,
			"transferId": transferId + rootId,
		})
	}

	return nil
}

func emitTransferFailedEvent(ctx context.Context, peerId string, rootEntryId string, rootEntryName string, transferId string) {
	runtime.EventsEmit(ctx, "transfer-failed", map[string]string{
		"id":         rootEntryId,
		"peerId":     peerId,
		"file":       rootEntryName,
		"transferId": transferId + rootEntryId,
	})
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

func (p *PeerSession) waitForPermission(transferId string) <-chan bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan bool)
	p.fileSendingPermissions[transferId] = ch

	log.Printf("Waiting for permission %v %v\n", transferId, p.fileSendingPermissions[transferId])

	return ch
}

func showFileResults(ch chan FileResult) {
	log.Printf("Showing file results")
	for fileResult := range ch {
		log.Printf("Showing file results %v", fileResult)
	}
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
