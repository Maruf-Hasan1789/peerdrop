package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	transport2 "github.com/Maruf-Hasan1789/peerdrop/internal/transport"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transport/pb"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/google/uuid"
	"github.com/labstack/gommon/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"google.golang.org/protobuf/proto"
)

type chunkedFile struct {
	file           *os.File
	fileMeta       *pb.FileMeta
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

	onFileOffer                func(peerId string, transferId string, rootEntry []*pb.RootEntry, totalSize int64)
	onTransferStart            func(peerId string, transferId string, rootId string, rootName string)
	onTransferCompletion       func(peerId string, transferId string, rootId string, rootName string)
	onTransferProgress         func(peerId string, transferId, rootId string, rootName string, progress float64)
	onTransferError            func(peerId string, transferId string, rootId string, rootName string, err error)
	onTransferPermissionDenied func(peerId string, transferId string, rootEntries []*pb.RootEntry)

	//message handlers map
	handlers               map[pb.MessageType]func(msg *pb.Message, payload []byte, downloadPath string)
	fileSendingPermissions map[string]chan *pb.Control
	mu                     sync.Mutex
	activeTransfers        map[string]context.CancelFunc

	rootPaths             map[string]string
	rootEntries           map[string]*pb.RootEntry
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
		handlers:               make(map[pb.MessageType]func(msg *pb.Message, payload []byte, downloadPath string)),
		lastSeen:               time.Now(),
		fileSendingPermissions: make(map[string]chan *pb.Control),
		rootPaths:              make(map[string]string),
		rootEntries:            make(map[string]*pb.RootEntry),
		rootReceivingProgress:  make(map[string]*RootProgress),
		rootSendingProgress:    make(map[string]*RootProgress),
	}

	p.handlers[pb.MessageType_MESSAGE_TYPE_CONTROL] = p.handleControl
	p.handlers[pb.MessageType_MESSAGE_TYPE_CHUNK] = p.handleChunk
	p.handlers[pb.MessageType_MESSAGE_TYPE_HANDSHAKE] = p.handleHandshake
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

func (p *PeerSession) OnFileOffer(fn func(peerId string, transferId string, rootEntries []*pb.RootEntry, totalSize int64)) {
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
func (p *PeerSession) OnTransferPermissionDenied(fn func(peerId string, transferId string, rootEntries []*pb.RootEntry)) {
	p.onTransferPermissionDenied = fn
}
func (p *PeerSession) readLoop(downloadPath string) {
	defer close(p.done)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			header, payload, err := p.conn.Receive()

			if err != nil {
				if p.ctx.Err() == nil {
					log.Printf("Peer disconnected during reading %v\n", err)
					p.handleDisconnect(err)
					return
				}

				log.Printf("Peer session stopped due to context cancellation\n")
			}

			p.handleMessage(header, payload, downloadPath)
		}
	}
}

func (p *PeerSession) handleMessage(header []byte, payload []byte, downloadPath string) {
	msg := &pb.Message{}

	if err := proto.Unmarshal(header, msg); err != nil {
		if p.onError != nil {
			p.onError(fmt.Errorf("invalid message: %w", err))
		}
		return
	}

	handler, ok := p.handlers[msg.GetType()]

	if ok {
		handler(msg, payload, downloadPath)
	} else {
		if p.onError != nil {
			p.onError(fmt.Errorf("unknown message type: %s", msg.GetType()))
		}
	}
}

func (p *PeerSession) handleControl(msg *pb.Message, payload []byte, downloadPath string) {

	//log.Printf("Handling Control %v\n", msg)
	control := msg.GetControl()
	action := control.GetAction()

	if action == pb.ControlAction_CONTROL_ACTION_HANDSHAKE_ACK {
		p.mu.Lock()

		//log.Printf("Message ID %v", msg.GetId())

		ch, ok := p.fileSendingPermissions[msg.GetId()]
		p.mu.Unlock()

		if !ok {
			log.Printf("File Sending Permissions channel not found for %v\n", msg.GetId())
		}

		delete(p.fileSendingPermissions, msg.GetId())

		if ch == nil {
			log.Printf("Permission channel is missing")
		}

		ch <- control
	}
}

func (p *PeerSession) Send(msg *pb.Message, payload []byte) error {
	header, err := proto.Marshal(msg)

	if err != nil {
		log.Printf("Error while sending msg = %v %v\n", msg, err)
		return err
	}

	return p.conn.Send(header, payload)
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
		files := rootEntry.GetFiles()
		for _, file := range files {
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
		if control.GetMode() == pb.PermissionMode_PERMISSION_MODE_NONE {
			//log.Printf("Permission is denied\n")

			if p.onTransferPermissionDenied != nil {
				p.onTransferPermissionDenied(p.peer.ID, transferId, rootEntries)
			}

			ctx.Done()
		} else if control.GetMode() == pb.PermissionMode_PERMISSION_MODE_ALL {
			var wg sync.WaitGroup
			startingTime := time.Now()
			for _, rootEntry := range rootEntries {
				wg.Add(1)
				rootPath := p.rootPaths[rootEntry.GetId()]
				//log.Printf("Here rootPath %v\n", rootPath)
				go p.SendRootEntry(transferId, rootEntry, rootPath, &wg)
			}
			wg.Wait()
			totalTime := time.Since(startingTime).Seconds()
			log.Printf("Here total time %v seconds\n", totalTime)
		} else {
			//log.Printf("Partial Permission is allowed\n")

			allowedRootID := make(map[string]bool)

			for _, file := range control.Files {
				if file.Allowed {
					allowedRootID[file.GetFileId()] = true
				} else {
					allowedRootID[file.GetFileId()] = false
				}
			}

			var wg sync.WaitGroup
			startingTime := time.Now()
			for _, rootEntry := range rootEntries {

				isAllowed, ok := allowedRootID[rootEntry.GetId()]
				if !ok || !isAllowed {
					continue
				}
				wg.Add(1)
				rootPath := p.rootPaths[rootEntry.GetId()]
				go p.SendRootEntry(transferId, rootEntry, rootPath, &wg)
			}

			wg.Wait()
			totalTime := time.Since(startingTime).Seconds()
			log.Printf("Here total time %v seconds\n", totalTime)
			//close(fileResultCh)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (p *PeerSession) SendRootEntry(transferId string, rootEntry *pb.RootEntry, rootPath string, wg *sync.WaitGroup) {
	defer wg.Done()

	for _, file := range rootEntry.Files {
		err := p.Sendfile(transferId, file, rootPath, rootEntry)
		if err != nil {
			log.Printf("Error sending file %v error : %v\n", file.Path, err)
			return
		}
	}
}

func (p *PeerSession) SendHandshakesForFiles(rootEntries []*pb.RootEntry, transferId string) error {
	var totalSize int64 = 0

	for _, rootEntry := range rootEntries {
		totalSize += rootEntry.Size
	}

	handshake := &pb.Handshake{
		TotalSize: totalSize,
		ChunkSize: int32(p.config.ChunkSize),
		Roots:     rootEntries,
	}

	fileOffer := &pb.Message{
		Version: int32(pb.Protocol_VERSION_V1),
		Type:    pb.MessageType_MESSAGE_TYPE_HANDSHAKE,
		Id:      transferId,
		Payload: &pb.Message_Handshake{
			Handshake: handshake,
		},
	}

	err := p.Send(fileOffer, nil)

	return err
}

func (p *PeerSession) Sendfile(transferId string, fileMeta *pb.FileMeta, rootPath string, rootEntry *pb.RootEntry) error {
	peerId := p.peer.ID
	filePath := rootPath

	if rootEntry.GetType() == pb.RootEntryType_ROOT_ENTRY_TYPE_DIRECTORY {
		filePath = filepath.Join(rootPath, fileMeta.Path)
	}

	//log.Printf("File Path of rootEntry Type = %v rootPath = %v FileMetaPath = %v\n filePath = %v\n", rootEntry.Type, rootPath, fileMeta.Path, filePath)
	f, err := os.Open(filePath)

	if err != nil {
		log.Printf("Error opening file %v: %v", fileMeta.Path, err)

		if p.onTransferError != nil {
			p.onTransferError(peerId, transferId, rootEntry.GetId(), rootEntry.GetName(), err)
		}

		return err
	}

	defer f.Close()

	fi, _ := f.Stat()

	chunkSize := p.config.ChunkSize

	totalChunks := int((fi.Size() + int64(chunkSize) - 1) / int64(chunkSize))

	log.Printf("total chunks %v\n", totalChunks)

	buf := make([]byte, chunkSize)

	rootId := rootEntry.GetId()

	rootSendProgress, ok := p.rootSendingProgress[rootId]

	if !ok {
		rootSendProgress = &RootProgress{
			RootID:           rootId,
			TotalBytes:       rootEntry.GetSize(),
			TransferredBytes: 0,
			LastEmit:         time.Now(),
		}

		p.rootSendingProgress[rootId] = rootSendProgress

		if p.onTransferStart != nil {
			p.onTransferStart(peerId, transferId, rootId, rootEntry.Name)
		}
	}
	//log.Printf("Here total Chunks %v", totalChunks)
	hash := sha256.New()
	for i := 0; i < totalChunks; i++ {
		n, err := f.Read(buf)

		if err != nil && err != io.EOF {
			p.onTransferError(peerId, transferId, rootId, rootEntry.Name, err)
			return err
		}
		hash.Reset()
		hash.Write(buf[:n])
		checkSum := hash.Sum(nil)
		//log.Printf("Check Sum %v\n", hex.EncodeToString(checkSum))
		chunk := &pb.Chunk{
			FileId:   fileMeta.GetId(),
			RootId:   rootEntry.GetId(),
			Index:    int32(i),
			Total:    int32(totalChunks),
			Offset:   int64(i * chunkSize),
			Size:     int64(n),
			Checksum: hex.EncodeToString(checkSum),
		}

		msg := &pb.Message{
			Version: int32(pb.Protocol_VERSION_V1),
			Type:    pb.MessageType_MESSAGE_TYPE_CHUNK,
			Id:      transferId,
			Payload: &pb.Message_Chunk{
				Chunk: chunk,
			},
		}

		if err := p.Send(msg, buf[:n]); err != nil {
			log.Printf("Here on line 403\n")
			p.onTransferError(peerId, transferId, rootId, rootEntry.Name, err)
			return err
		}

		rootSendProgress.mu.Lock()
		rootSendProgress.TransferredBytes += int64(n)
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

func (p *PeerSession) handleChunk(msg *pb.Message, payload []byte, downloadPath string) {
	chunk := msg.Payload.(*pb.Message_Chunk).Chunk
	incomingFileId := getIncomingFileId(msg.GetId(), chunk.GetRootId(), chunk.GetFileId())
	f, ok := incomingFiles[incomingFileId]

	if !ok {
		log.Printf("File was not mentioned in handshake\n")
		return
	}

	fileName := f.fileMeta.Path
	fileName = strings.ReplaceAll(fileName, "/", string(filepath.Separator))
	fileName = strings.ReplaceAll(fileName, `\`, string(filepath.Separator))
	fileName = strings.TrimPrefix(fileName, " ")
	if fileName == "" {
		log.Printf("File name is empty\n")
		return
	}
	if filepath.IsAbs(fileName) {
		log.Printf("Ignoring absolute file %v\n", fileName)
		return
	}

	if f.file == nil {

		rootEntry, ok := p.rootEntries[f.rootId]
		if !ok {
			log.Printf("No root entry for file %v\n", f.fileMeta.Path)
			return
		}

		var filePath string
		var baseDir string
		var cleanPath string
		if rootEntry.Type == pb.RootEntryType_ROOT_ENTRY_TYPE_FILE {
			baseDir = filepath.Join(downloadPath, rootEntry.Name)
		} else {
			baseDir = downloadPath
		}

		baseDir = filepath.Clean(baseDir)

		filePath = filepath.Join(baseDir, fileName)
		cleanPath = filepath.Clean(filePath)

		rel, err := filepath.Rel(baseDir, cleanPath)

		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			log.Printf("Invalid path %v\n", cleanPath)
			return
		}

		err = os.MkdirAll(filepath.Dir(cleanPath), 0755)
		if err != nil {
			log.Printf("Error creating directory: %v\n", err)
			return
		}

		file, err := openFile(cleanPath)
		if err != nil {
			log.Printf("Error opening file %v\n", err)
			return
		}

		f.file = file

		rProgress, ok := p.rootReceivingProgress[rootEntry.GetId()]

		if !ok {
			log.Printf("Here rProgress not present")
			rProgress = &RootProgress{
				RootID:           rootEntry.GetId(),
				TotalBytes:       rootEntry.GetSize(),
				TransferredBytes: 0,
				LastEmit:         time.Now(),
			}
			p.rootReceivingProgress[rootEntry.GetId()] = rProgress

			runtime.EventsEmit(p.ctx, "receiving-started", map[string]string{
				"id":            f.rootId,
				"file":          p.rootEntries[f.rootId].GetName(),
				"totalChunks":   strconv.FormatInt(rProgress.TotalBytes, 10),
				"totalReceived": "0",
			})
		}
	}

	hash := sha256.New()
	hash.Write(payload)

	finalHash := hash.Sum(nil)
	receivedChecksum := hex.EncodeToString(finalHash)
	//log.Printf("Calculated CheckSum %v Received Checksum %v\n", receivedChecksum, chunk.GetChecksum())
	if receivedChecksum == chunk.GetChecksum() && f.receivedChunks[int(chunk.GetIndex())] == false {
		offset := chunk.GetOffset()
		_, err := f.file.WriteAt(payload, offset)

		if err != nil {
			log.Printf("Error writing to file %v: %v", f.fileMeta.Path, err)
			return
		}
		f.receivedChunks[int(chunk.GetIndex())] = true
		f.receivedCount++
	} else {
		log.Printf("Received CheckSum %v is not equal to calculated Checksum %v\n", receivedChecksum, chunk.GetChecksum())
		return
	}

	rProgress := p.rootReceivingProgress[f.rootId]

	rProgress.mu.Lock()
	rProgress.TransferredBytes += int64(len(payload))
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
		//log.Printf("File: %v is received successfully\n", fileName)
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
	//log.Printf("Peer disconnected during reading %v %v\n", p.peer.Name, err)

	if p.onDisconnected != nil {
		log.Printf("Peer On Disconnected Provided: %v\n", p.peer.Name)
		p.onDisconnected(p.peer)
	}

	if p.onError != nil && err != io.EOF {
		p.onError(err)
	}
}

func (p *PeerSession) handleHandshake(msg *pb.Message, payload []byte, path string) {
	//log.Printf("Handling Handshake %v\n", msg)
	handshake := msg.GetPayload().(*pb.Message_Handshake).Handshake
	roots := handshake.GetRoots()

	for _, rootEntry := range roots {
		p.rootEntries[rootEntry.GetId()] = rootEntry

		for _, file := range rootEntry.Files {
			incomingFileId := getIncomingFileId(msg.GetId(), rootEntry.GetId(), file.GetId())
			incomingFiles[incomingFileId] = &chunkedFile{
				totalChunks:    (file.Size + int64(handshake.GetChunkSize()-1)) / int64(handshake.GetChunkSize()),
				chunkSize:      int(handshake.GetChunkSize()),
				fileMeta:       file,
				receivedCount:  0,
				receivedChunks: make(map[int]bool),
				status:         "Initiated",
				lastEmit:       time.Now(),
				rootId:         rootEntry.GetId(),
			}
		}
	}

	if p.onFileOffer != nil {
		p.onFileOffer(p.peer.ID, msg.GetId(), roots, handshake.GetTotalSize())
	}
}

func getIncomingFileId(transferId string, rootId string, fileId string) string {
	return fmt.Sprintf("%v|%v|%v", transferId, rootId, fileId)
}

func (p *PeerSession) waitForPermission(transferId string) <-chan *pb.Control {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan *pb.Control)
	p.fileSendingPermissions[transferId] = ch

	log.Printf("Waiting for permission %v %v\n", transferId, p.fileSendingPermissions[transferId])

	return ch
}

func (p *PeerSession) getRootEntriesFromFilePaths(paths []string) []*pb.RootEntry {
	var rootEntries []*pb.RootEntry
	for _, path := range paths {
		rootEntry, err := getRootEntryFromPath(path)
		if err != nil {
			log.Printf("Error getting file info: %v\n", err)
			continue
		}

		p.rootPaths[rootEntry.GetId()] = path
		rootEntries = append(rootEntries, rootEntry)
	}

	return rootEntries
}

func getRootEntryFromPath(path string) (*pb.RootEntry, error) {
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

		return &pb.RootEntry{
			Id:   rootId,
			Name: fileName,
			Type: pb.RootEntryType_ROOT_ENTRY_TYPE_FILE,
			Size: fileSize,
			Files: []*pb.FileMeta{{
				Id:   fileId,
				Path: fileName,
				Size: fileSize},
			},
		}, nil
	}

	rootEntry := &pb.RootEntry{
		Id:    rootId,
		Name:  filepath.Base(path),
		Type:  pb.RootEntryType_ROOT_ENTRY_TYPE_DIRECTORY,
		Files: []*pb.FileMeta{},
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

		rootEntry.Files = append(rootEntry.Files, &pb.FileMeta{
			Id:   fmt.Sprintf("%v_%v", filepath.Base(relativePath), time.Now().UnixNano()),
			Path: relativePath,
			Size: info.Size(),
		})
		totalSize += info.Size()

		return nil
	})

	rootEntry.Size = totalSize

	return rootEntry, err
}
