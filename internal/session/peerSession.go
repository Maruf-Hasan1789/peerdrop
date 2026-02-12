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
	chunkSize      int64
	receivedCount  int64
	receivedChunks map[int]bool
	status         string
	lastEmit       time.Time
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
	fileReceivedListeners []func(peerId string, fileId string, fileName string)
	onFileOffer           func(peerId string, transferId string, files []protocol.FileMeta, totalSize int64)

	//message handlers map
	handlers               map[protocol.MessageType]func(ctx context.Context, msg protocol.Message, downloadPath string)
	fileSendingPermissions map[string]chan bool
	mu                     sync.Mutex
	activeTransfers        map[string]context.CancelFunc
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
		handlers:               make(map[protocol.MessageType]func(ctx context.Context, msg protocol.Message, downloadPath string)),
		lastSeen:               time.Now(),
		fileSendingPermissions: make(map[string]chan bool),
	}

	p.handlers["file"] = p.handleFile
	p.handlers["CONTROL"] = p.handleControl
	p.handlers["CHUNK"] = p.handleChunk
	p.handlers["HANDSHAKE"] = p.handleHandshake
	return p
}

func (p *PeerSession) Start(downloadPath string) {
	go p.readLoop(p.ctx, downloadPath)
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

func (p *PeerSession) OnFileReceived(fn func(peerId string, fileId string, fileName string)) {
	p.fileReceivedListeners = append(p.fileReceivedListeners, fn)
}

func (p *PeerSession) OnFileOffer(fn func(peerId string, transferId string, files []protocol.FileMeta, totalSize int64)) {
	p.onFileOffer = fn
}

func (p *PeerSession) readLoop(ctx context.Context, downloadPath string) {
	defer close(p.done)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			data, err := p.conn.Receive()

			if err != nil {
				if ctx.Err() == nil {
					log.Printf("Peer disconnected during reading %v\n", err)
					p.handleDisconnect(err)
					return
				}

				log.Printf("Peer session stopped due to context cancellation\n")
			}

			p.handleMessage(ctx, data, downloadPath)
		}
	}
}

func (p *PeerSession) handleMessage(ctx context.Context, data []byte, downloadPath string) {
	var msg protocol.Message

	if err := json.Unmarshal(data, &msg); err != nil {
		if p.onError != nil {
			p.onError(fmt.Errorf("invalid message: %w", err))
		}
		return
	}

	handler, ok := p.handlers[msg.Type]

	if ok {
		handler(ctx, msg, downloadPath)
	} else {
		if p.onError != nil {
			p.onError(fmt.Errorf("unknown message type: %s", msg.Type))
		}
	}
}

func (p *PeerSession) handleText(ctx context.Context, msg protocol.Message, downloadPath string) {
	log.Printf("Peer says: %v", string(msg.Type))
}

func (p *PeerSession) handleControl(ctx context.Context, msg protocol.Message, downloadPath string) {

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

func (p *PeerSession) handleFile(ctx context.Context, msg protocol.Message, downloadPath string) {

	/*fileName := filepath.Base(msg.Name)

	if err := os.WriteFile(filepath.Join(downloadPath, fileName), msg.Data, 0644); err != nil {
		if p.onError != nil {
			p.onError(fmt.Errorf("cannot save file: %w", err))
		}
		return
	}

	log.Printf("File received %v\n", fileName)

	for _, fn := range p.fileReceivedListeners {
		fn(p.peer.ID, msg.Id, fileName)
	}

	*/
}

func (p *PeerSession) Send(msg protocol.Message) error {
	//log.Printf("Sending file %v\n", msg)
	payload, err := json.Marshal(msg)

	if err != nil {
		return err
	}

	return p.conn.Send(payload)
}

type FileInfo struct {
	FileId   string
	FileName string
	FilePath string
	FileSize int64
	Checksum string
}

type FileResult struct {
	FileId   string
	FileName string
	Error    error
}

func (p *PeerSession) SendToPeer(ctx context.Context, transferId string, filePaths []string) error {
	permCh := p.waitForPermission(transferId)
	fileInfos := getFileInfos(filePaths)
	log.Printf("File Infos %v\n", fileInfos)

	err := p.SendHandshakesForFiles(ctx, fileInfos, 1024*1024, transferId)

	if err != nil {
		log.Printf("Error sending handshake for files: %v\n", err)
		return err
	}

	ctx, _ = context.WithCancel(ctx)

	select {
	case isAllowed := <-permCh:
		if !isAllowed {
			log.Printf("Permission is denied\n")
		} else {
			log.Printf("Permission is allowed\n")
			fileResultCh := make(chan FileResult, len(fileInfos))
			wg := sync.WaitGroup{}
			go showFileResults(fileResultCh)
			for _, fileInfo := range fileInfos {
				wg.Add(1)
				go p.Sendfile(ctx, transferId, fileInfo, 1024*1024, fileResultCh, &wg)
			}

			wg.Wait()
			close(fileResultCh)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func showFileResults(ch chan FileResult) {
	log.Printf("Showing file results")
	for fileResult := range ch {
		log.Printf("Showing file results %v", fileResult)
	}
}

func getFileInfos(paths []string) []FileInfo {
	var fileInfos []FileInfo
	for _, path := range paths {
		fileInfo, err := getFileInfoFromPath(path)
		if err != nil {
			log.Printf("Error getting file info: %v\n", err)
			continue
		}
		fileInfos = append(fileInfos, *fileInfo)
	}

	return fileInfos
}

func (p *PeerSession) SendHandshakesForFiles(ctx context.Context, fileInfos []FileInfo, chunkSize int, transferId string) error {
	log.Printf("Sending Handshakes for files %v\n", fileInfos)

	fileOffer := protocol.Message{
		Version:   protocol.ProtocolVersion,
		Type:      protocol.TypeHandshake,
		ID:        transferId,
		Handshake: &protocol.Handshake{},
	}

	var files []protocol.FileMeta

	var totalSize int64 = 0

	for _, fileInfo := range fileInfos {
		files = append(files, protocol.FileMeta{
			ID:       fileInfo.FileId,
			Path:     fileInfo.FileName,
			Size:     fileInfo.FileSize,
			CheckSum: fileInfo.Checksum,
		})
		totalSize += fileInfo.FileSize
	}

	fileOffer.Handshake.Files = files
	fileOffer.Handshake.TotalSize = totalSize
	fileOffer.Handshake.ChunkSize = int64(chunkSize)

	err := p.Send(fileOffer)

	return err
}

func getFileInfoFromPath(path string) (*FileInfo, error) {
	f, err := os.Open(path)

	if err != nil {
		log.Printf("Error opening file %v: %v", path, err)
		return nil, err
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Printf("Error closing file %v: %v\n", path, err)
		}
	}(f)

	fileInfo, err := f.Stat()

	if err != nil {
		log.Printf("Error stating file %v: %v", path, err)
		return nil, err
	}

	fileName := filepath.Base(path)
	fileId := fmt.Sprintf("%v_%v", filepath.Base(path), time.Now().UnixNano())
	fileSize := fileInfo.Size()
	checkSum, err := getFileChecksum(path)

	if err != nil {
		log.Printf("Error getting file checksum %v: %v", path, err)
		return nil, err
	}

	return &FileInfo{
		FileId:   fileId,
		FileName: fileName,
		FilePath: path,
		FileSize: fileSize,
		Checksum: hex.EncodeToString(checkSum),
	}, nil
}

func getFileChecksum(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Error opening file %v while calculating checksum: %v", path, err)
		return nil, err
	}

	defer file.Close()

	hash := sha256.New()

	if _, err := io.Copy(hash, file); err != nil {
		log.Printf("Error calculating checksum: %v", err)
		return nil, err
	}

	return hash.Sum(nil), nil
}

func (p *PeerSession) Sendfile(ctx context.Context, transferId string, fileInfo FileInfo, chunkSize int, fileResultCh chan FileResult, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Entering Sending LargeFile %v\n", fileInfo)

	peerId := p.peer.ID

	f, err := os.Open(fileInfo.FilePath)

	if err != nil {
		log.Printf("Error opening file %v: %v", fileInfo.FilePath, err)
		emitTransferFailedEvent(ctx, peerId, fileInfo.FileId, fileInfo.FileName, transferId)
		fileResultCh <- FileResult{
			FileId:   fileInfo.FileId,
			FileName: fileInfo.FileName,
			Error:    err,
		}
		return
	}

	defer f.Close()

	fi, _ := f.Stat()

	totalChunks := int((fi.Size() + int64(chunkSize) - 1) / int64(chunkSize))

	log.Printf("total chunks %v\n", totalChunks)

	buf := make([]byte, chunkSize)

	runtime.EventsEmit(ctx, "transfer-start", map[string]string{
		"id":         fileInfo.FileId,
		"peerId":     peerId,
		"fileName":   fileInfo.FileName,
		"transferId": transferId + fileInfo.FileId,
	})

	lastTime := time.Now()
	emitInterval := 1 * time.Second

	for i := 0; i < totalChunks; i++ {
		n, err := f.Read(buf)

		if err != nil && err != io.EOF {
			emitTransferFailedEvent(ctx, peerId, fileInfo.FileId, fileInfo.FileName, transferId)
			fileResultCh <- FileResult{
				FileId:   fileInfo.FileId,
				FileName: fileInfo.FileName,
				Error:    err,
			}
			return
		}

		hash := sha256.New()
		hash.Write(buf[:n])
		checkSum := hash.Sum(nil)

		chunk := &protocol.Chunk{
			FileID:   fileInfo.FileId,
			Index:    i,
			Total:    totalChunks,
			Offset:   int64(i * chunkSize),
			Size:     fileInfo.FileSize,
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
			emitTransferFailedEvent(ctx, peerId, fileInfo.FileId, fileInfo.FileName, transferId)
			fileResultCh <- FileResult{
				FileId:   fileInfo.FileId,
				FileName: fileInfo.FileName,
				Error:    err,
			}
			return
		}

		if time.Now().Sub(lastTime) > emitInterval {
			runtime.EventsEmit(ctx, "transfer-progress", map[string]string{
				"id":          fileInfo.FileId,
				"peerId":      peerId,
				"file":        fileInfo.FileName,
				"totalChunks": strconv.Itoa(totalChunks),
				"chunkIndex":  strconv.Itoa(msg.Chunk.Index),
				"transferId":  transferId + fileInfo.FileId,
			})
			lastTime = time.Now()
		}
	}

	runtime.EventsEmit(ctx, "transfer-complete", map[string]string{
		"id":         fileInfo.FileId,
		"peerId":     peerId,
		"file":       fileInfo.FileName,
		"transferId": transferId + fileInfo.FileId,
	})
}

func emitTransferFailedEvent(ctx context.Context, peerId string, fileId string, fileName string, transferId string) {
	runtime.EventsEmit(ctx, "transfer-failed", map[string]string{
		"id":         fileId,
		"peerId":     peerId,
		"file":       fileName,
		"transferId": transferId + fileId,
	})
}

func openFile(originalFileName string) (*os.File, error) {
	log.Printf("Opening file %v\n", originalFileName)
	dir := filepath.Dir(originalFileName)
	ext := filepath.Ext(originalFileName)
	base := filepath.Base(originalFileName[:len(originalFileName)-len(ext)])

	for i := 0; ; i++ {
		name := base + ext
		if i > 0 {
			name = fmt.Sprintf("%s(%d)%s", base, i, ext)
		}

		newPath := filepath.Join(dir, name)
		log.Printf("Opening file %v\n", newPath)
		file, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)

		if err == nil {
			log.Printf("Opened file %v\n", newPath)
			return file, nil
		}

		log.Printf("Here error in opening file %v\n", err)
	}
}

func (p *PeerSession) handleChunk(ctx context.Context, msg protocol.Message, downloadPath string) {
	incomingFileId := getIncomingFileId(msg.ID, msg.Chunk.FileID)
	f, ok := incomingFiles[incomingFileId]

	fileId := msg.Chunk.FileID

	if !ok {
		log.Printf("File was not mentioned in handshake\n")
		return
	}

	fileName := f.fileMeta.Path

	if f.file == nil {
		file, err := openFile(filepath.Join(downloadPath, f.fileMeta.Path))
		if err != nil {
			log.Printf("Error opening file %v\n", err)
			return
		}

		f.file = file
		/*err = f.file.Truncate(f.totalChunks * f.chunkSize)

		if err != nil {
			log.Printf("Error truncating file %v: %v", fileName, err)
			return
		}

		*/

		runtime.EventsEmit(ctx, "receiving-started", map[string]string{
			"id":            fileId,
			"file":          f.fileMeta.Path,
			"totalChunks":   strconv.Itoa(int(f.totalChunks)),
			"totalReceived": "0",
		})
		f.lastEmit = time.Now()
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
	}

	if time.Since(f.lastEmit) >= time.Second {
		runtime.EventsEmit(ctx, "receiving-progress", map[string]string{
			"id":            fileId,
			"file":          f.fileMeta.Path,
			"totalChunks":   strconv.Itoa(int(f.totalChunks)),
			"totalReceived": strconv.Itoa(int(f.receivedCount)),
		})
		f.lastEmit = time.Now()
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

		_ = f.file.Close()

		for _, fn := range p.fileReceivedListeners {
			fn(p.peer.ID, fileId, fileName)
		}

		log.Printf("Large file received %v\n", fileName)

		runtime.EventsEmit(ctx, "file-received", map[string]string{
			"id":       fileId,
			"fileName": fileName,
			"peer":     p.peer.UserName,
		})

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

func (p *PeerSession) handleHandshake(ctx context.Context, msg protocol.Message, path string) {
	log.Printf("Handling Handshake %v\n", msg)

	for _, file := range msg.Handshake.Files {
		incomingFileId := getIncomingFileId(msg.ID, file.ID)
		incomingFiles[incomingFileId] = &chunkedFile{
			totalChunks:    (file.Size + msg.Handshake.ChunkSize - 1) / msg.Handshake.ChunkSize,
			chunkSize:      msg.Handshake.ChunkSize,
			fileMeta:       file,
			receivedCount:  0,
			receivedChunks: make(map[int]bool),
			status:         "Initiated",
			lastEmit:       time.Now(),
		}
	}

	if p.onFileOffer != nil {
		p.onFileOffer(p.peer.ID, msg.ID, msg.Handshake.Files, msg.Handshake.TotalSize)
	}
}

func getIncomingFileId(transferId string, fileId string) string {
	return fmt.Sprintf("%v|%v", transferId, fileId)
}

func (p *PeerSession) waitForPermission(transferId string) <-chan bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan bool)
	p.fileSendingPermissions[transferId] = ch

	log.Printf("Waiting for permission %v %v\n", transferId, p.fileSendingPermissions[transferId])

	return ch
}
