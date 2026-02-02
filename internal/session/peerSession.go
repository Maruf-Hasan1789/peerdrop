package session

import (
	"context"
	"crypto/sha256"
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
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type chunkedFile struct {
	file          *os.File
	totalChunks   int
	receivedCount int
}

var incomingFiles = make(map[string]*chunkedFile)

type PeerSession struct {
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
	fileReceivedListeners []func(name string)
	onFileOffer           func(peerId string, fileName string, fileId string, FileStatus transfer.Status, chunkReceived int64, totalChunks int64)

	//message handlers map
	handlers               map[string]func(ctx context.Context, msg protocol.Message, downloadPath string)
	fileSendingPermissions map[string]chan bool
	mu                     sync.Mutex
	activeTransfers        map[string]context.CancelFunc
}

func NewPeerSession(ctx context.Context, conn transport.Connection) *PeerSession {
	ctx, cancel := context.WithCancel(ctx)
	p := &PeerSession{
		ctx:                    ctx,
		cancel:                 cancel,
		conn:                   conn,
		peer:                   conn.PeerInfo(),
		done:                   make(chan struct{}),
		connectedAt:            time.Now(),
		handlers:               make(map[string]func(ctx context.Context, msg protocol.Message, downloadPath string)),
		lastSeen:               time.Now(),
		fileSendingPermissions: make(map[string]chan bool),
	}

	p.handlers["file"] = p.handleFile
	p.handlers["text"] = p.handleText
	p.handlers["file-chunk"] = p.handleChunk
	p.handlers["file-offer"] = p.handleFileOffer
	p.handlers["file-permission"] = p.handleFilePermission
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

func (p *PeerSession) OnFileReceived(fn func(name string)) {
	p.fileReceivedListeners = append(p.fileReceivedListeners, fn)
}

func (p *PeerSession) OnFileOffer(fn func(peerId string, fileName string, fileId string, FileStatus transfer.Status, chunkReceived int64, totalChunks int64)) {
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
	log.Printf("Peer says: %v", string(msg.Data))
}

func (p *PeerSession) handleFile(ctx context.Context, msg protocol.Message, downloadPath string) {

	fileName := filepath.Base(msg.Name)

	if err := os.WriteFile(filepath.Join(downloadPath, fileName), msg.Data, 0644); err != nil {
		if p.onError != nil {
			p.onError(fmt.Errorf("cannot save file: %w", err))
		}
		return
	}

	log.Printf("File received %v\n", fileName)

	for _, fn := range p.fileReceivedListeners {
		fn(fileName)
	}
}

func (p *PeerSession) SendText(text string) error {
	msg := protocol.Message{
		Type: "text",
		Data: []byte(text),
	}
	return p.Send(msg)
}

func (p *PeerSession) SendFile(path string) error {
	data, err := os.ReadFile(path)

	if err != nil {
		return err
	}

	msg := protocol.Message{
		Type: "file",
		Name: filepath.Base(path),
		Data: data,
	}

	return p.Send(msg)
}

func (p *PeerSession) Send(msg protocol.Message) error {
	log.Printf("Sending file %v\n", msg.ChunkIndex)
	payload, err := json.Marshal(msg)

	if err != nil {
		return err
	}

	return p.conn.Send(payload)
}

func (p *PeerSession) SendLargeFile(ctx context.Context, path string, chunkSize int, fileId string) error {
	log.Printf("Entering Sending LargeFile %v\n", path)

	transferId := fmt.Sprintf("%v_%v", filepath.Base(path), time.Now().UnixNano())
	fileName := filepath.Base(path)
	peerId := p.peer.ID

	f, err := os.Open(path)

	if err != nil {
		log.Printf("Error opening file %v: %v", path, err)
		emitTransferFailedEvent(ctx, peerId, fileId, fileName, transferId)
		return err
	}

	defer f.Close()

	fi, _ := f.Stat()

	totalChunks := int((fi.Size() + int64(chunkSize) - 1) / int64(chunkSize))

	log.Printf("total chunks %v\n", totalChunks)

	buf := make([]byte, chunkSize)

	log.Printf("File Id %v\n", fileId)

	fileOffer := protocol.Message{
		Type:        "file-offer",
		Name:        fileName,
		Id:          fileId,
		Data:        nil,
		TotalChunks: totalChunks,
		ChunkSize:   chunkSize,
		Checksum:    "checkSum",
	}

	permCh := p.waitForPermission(fileId)

	if err := p.Send(fileOffer); err != nil {
		log.Printf("Error sending file-offer: %v", err)
		emitTransferFailedEvent(ctx, peerId, fileId, fileName, transferId)
		return err
	}

	ctx, _ = context.WithCancel(p.ctx)
	select {
	case allowed := <-permCh:
		if !allowed {
			emitTransferFailedEvent(ctx, peerId, fileId, fileName, transferId)
			return fmt.Errorf("permission denied by User")
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	runtime.EventsEmit(ctx, "transfer-start", map[string]string{
		"id":         fileId,
		"peerId":     peerId,
		"fileName":   fileName,
		"transferId": transferId,
	})

	for i := 0; i < totalChunks; i++ {
		n, err := f.Read(buf)

		if err != nil && err != io.EOF {
			emitTransferFailedEvent(ctx, peerId, fileId, path, transferId)
			return err
		}
		hash := sha256.New()
		hash.Write(buf[:n])
		checkSum := hash.Sum(nil)

		msg := protocol.Message{
			Type:        "file-chunk",
			Name:        filepath.Base(path),
			Id:          fileId,
			Data:        buf[:n],
			ChunkIndex:  i,
			TotalChunks: totalChunks,
			Checksum:    fmt.Sprintf("%x", checkSum),
			Allowed:     true,
		}

		if err := p.Send(msg); err != nil {
			emitTransferFailedEvent(ctx, peerId, fileId, path, transferId)
			return err
		}

		runtime.EventsEmit(ctx, "transfer-progress", map[string]string{
			"id":          fileId,
			"peerId":      peerId,
			"file":        fileName,
			"totalChunks": strconv.Itoa(totalChunks),
			"chunkIndex":  strconv.Itoa(msg.ChunkIndex),
			"transferId":  transferId,
		})
	}

	runtime.EventsEmit(ctx, "transfer-complete", map[string]string{
		"id":         fileId,
		"peerId":     peerId,
		"file":       fileName,
		"transferId": transferId,
	})

	return nil
}

func emitTransferFailedEvent(ctx context.Context, peerId string, fileId string, fileName string, transferId string) {
	runtime.EventsEmit(ctx, "transfer-failed", map[string]string{
		"id":         fileId,
		"peerId":     peerId,
		"file":       fileName,
		"transferId": transferId,
	})
}

func (p *PeerSession) handleChunk(ctx context.Context, msg protocol.Message, downloadPath string) {
	f, ok := incomingFiles[msg.Name]

	fileId := msg.Id

	if !ok {
		file, err := os.OpenFile(filepath.Join(downloadPath, msg.Name), os.O_CREATE|os.O_RDWR, 0644)

		if err != nil {
			log.Printf("Error opening file %v: %v", msg.Name, err)
			return
		}

		f = &chunkedFile{
			totalChunks:   msg.TotalChunks,
			receivedCount: 0,
			file:          file,
		}

		err = f.file.Truncate(int64(msg.TotalChunks) * int64(1024*1024))

		if err != nil {
			log.Printf("Error truncating file %v: %v", msg.Name, err)
			return
		}

		incomingFiles[msg.Name] = f
		runtime.EventsEmit(ctx, "receiving-started", map[string]string{
			"id":            fileId,
			"file":          msg.Name,
			"totalChunks":   strconv.Itoa(msg.TotalChunks),
			"totalReceived": "0",
		})

		p.lastSeen = time.Now()
	}

	hash := sha256.New()
	hash.Write(msg.Data)

	finalHash := hash.Sum(nil)
	receivedChecksum := fmt.Sprintf("%x", finalHash)

	log.Printf("Received Checksum %v Calculated checkSum %v\n", receivedChecksum, msg.Checksum)
	log.Printf("File Id %v\n", fileId)

	if receivedChecksum == msg.Checksum {

		chunkSize := 1024 * 1024

		offset := int64(msg.ChunkIndex) * int64(chunkSize)

		_, err := f.file.WriteAt(msg.Data, offset)

		if err != nil {
			log.Printf("Error writing to file %v: %v", msg.Name, err)
			return
		}
		f.receivedCount++
	} else {
		log.Printf("Received CheckSum %v is not equal to calculated Checksum %v\n", receivedChecksum, msg.Checksum)
	}

	if time.Since(p.lastSeen) >= time.Second {
		runtime.EventsEmit(ctx, "receiving-progress", map[string]string{
			"id":            fileId,
			"file":          msg.Name,
			"totalChunks":   strconv.Itoa(msg.TotalChunks),
			"totalReceived": strconv.Itoa(f.receivedCount),
		})
		p.lastSeen = time.Now()
	}

	//check if all chunks received

	complete := false

	if f.receivedCount == msg.TotalChunks {
		complete = true
	}

	if complete {
		fileName := filepath.Base(msg.Name)

		log.Printf("FileName: %v\n", fileName)
		err := f.file.Truncate(int64(msg.TotalChunks-1)*int64(1024*1024) + int64(len(msg.Data)))
		if err != nil {
			log.Printf("Error truncating file %v: %v", msg.Name, err)
			return
		}

		hash := sha256.New()
		_, err = f.file.Seek(0, io.SeekStart)

		if err != nil {
			log.Printf("Error seeking to file %v: %v", msg.Name, err)
			return
		}

		if _, err := io.Copy(hash, f.file); err != nil {
			log.Printf("Error reading file %v: %v", msg.Name, err)
		}

		receivedFileCheckSum := fmt.Sprintf("%x", hash.Sum(nil))

		log.Printf("FileCheckSum: %v\n", receivedFileCheckSum)

		f.file.Close()

		for _, fn := range p.fileReceivedListeners {
			fn(fileName)
		}

		log.Printf("Large file received %v\n", fileName)

		//time.Sleep(1 * time.Second)

		runtime.EventsEmit(ctx, "file-received", map[string]string{
			"id":   fileId,
			"file": fileName,
		})

		delete(incomingFiles, msg.Name)
	}
}

func (p *PeerSession) GetPeerInfo() discovery.Peer {
	return p.peer
}

func (p *PeerSession) handleDisconnect(err error) {
	log.Printf("Peer disconnected during reading %v %v\n", p.peer.Name, err)

	if p.onDisconnected != nil {
		p.onDisconnected(p.peer)
	}

	if p.onError != nil && err != io.EOF {
		p.onError(err)
	}
}

func (p *PeerSession) handleFileOffer(ctx context.Context, msg protocol.Message, path string) {
	log.Printf("HandleFile Offer %v\n", msg)

	if p.onFileOffer != nil {
		p.onFileOffer(p.peer.ID, msg.Name, msg.Id, transfer.InProgress, 0, int64(msg.TotalChunks))
	}
}

func (p *PeerSession) handleFilePermission(ctx context.Context, msg protocol.Message, path string) {
	p.mu.Lock()
	ch := p.fileSendingPermissions[msg.Id]
	p.mu.Unlock()

	delete(p.fileSendingPermissions, msg.Id)

	if ch == nil {
		log.Printf("Permission Channel is missing\n")
	}

	ch <- msg.Allowed
}

func (p *PeerSession) waitForPermission(fileId string) <-chan bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan bool)
	p.fileSendingPermissions[fileId] = ch
	return ch
}
