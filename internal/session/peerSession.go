package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type chunkedFile struct {
	totalChunks   int
	received      [][]byte
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

	//message handlers map
	handlers map[string]func(ctx context.Context, msg protocol.Message, downloadPath string)
}

func NewPeerSession(ctx context.Context, conn transport.Connection) *PeerSession {
	ctx, cancel := context.WithCancel(ctx)
	p := &PeerSession{
		ctx:         ctx,
		cancel:      cancel,
		conn:        conn,
		peer:        conn.PeerInfo(),
		done:        make(chan struct{}),
		connectedAt: time.Now(),
		handlers:    make(map[string]func(ctx context.Context, msg protocol.Message, downloadPath string)),
		lastSeen:    time.Now(),
	}

	p.handlers["file"] = p.handleFile
	p.handlers["text"] = p.handleText
	p.handlers["file-chunk"] = p.handleChunk
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

func (p *PeerSession) readLoop(ctx context.Context, downloadPath string) {
	defer close(p.done)

	for {
		data, err := p.conn.Receive()

		if err != nil {
			p.handleDisconnect(err)
			return
		}

		p.handleMessage(ctx, data, downloadPath)
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
	return p.send(msg)
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

	return p.send(msg)
}

func (p *PeerSession) send(msg protocol.Message) error {
	log.Printf("Sending file %v\n", msg.ChunkIndex)
	payload, err := json.Marshal(msg)

	if err != nil {
		return err
	}

	return p.conn.Send(payload)
}

func (p *PeerSession) SendLargeFile(ctx context.Context, path string, chunkSize int) error {
	log.Printf("Entering Sending LargeFile %v\n", path)

	f, err := os.Open(path)

	if err != nil {
		log.Printf("Error opening file %v: %v", path, err)
		return err
	}

	defer f.Close()

	fi, _ := f.Stat()

	totalChunks := int((fi.Size() + int64(chunkSize) - 1) / int64(chunkSize))

	log.Printf("total chunks %v\n", totalChunks)

	buf := make([]byte, chunkSize)
	fileId := fmt.Sprintf("%v-%v", path, time.Now().Unix())

	runtime.EventsEmit(ctx, "transfer-start", map[string]string{
		"id":       fileId,
		"fileName": path,
	})

	for i := 0; i < totalChunks; i++ {
		n, err := f.Read(buf)

		if err != nil && err != io.EOF {
			emitTransferFailedEvent(ctx, fileId, path)
			return err
		}

		msg := protocol.Message{
			Type:        "file-chunk",
			Name:        filepath.Base(path),
			Data:        buf[:n],
			ChunkIndex:  i,
			TotalChunks: totalChunks,
		}

		if err := p.send(msg); err != nil {
			emitTransferFailedEvent(ctx, fileId, path)
			return err
		}

		runtime.EventsEmit(ctx, "transfer-progress", map[string]string{
			"id":          fileId,
			"file":        path,
			"totalChunks": strconv.Itoa(totalChunks),
			"chunkIndex":  strconv.Itoa(msg.ChunkIndex),
		})
	}

	runtime.EventsEmit(ctx, "transfer-complete", map[string]string{
		"id":   fileId,
		"file": path,
	})
	return nil
}

func emitTransferFailedEvent(ctx context.Context, fileId string, path string) {
	runtime.EventsEmit(ctx, "transfer-failed", map[string]string{
		"id":   fileId,
		"file": path,
	})
}

func (p *PeerSession) handleChunk(ctx context.Context, msg protocol.Message, downloadPath string) {
	f, ok := incomingFiles[msg.Name]

	fileId := fmt.Sprintf("%v-%v", msg.Name, time.Now().Unix())

	if !ok {
		f = &chunkedFile{
			totalChunks:   msg.TotalChunks,
			received:      make([][]byte, msg.TotalChunks),
			receivedCount: 0,
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

	if f.received[msg.ChunkIndex] == nil {
		f.received[msg.ChunkIndex] = msg.Data
		f.receivedCount++

		if time.Since(p.lastSeen) >= time.Second {
			runtime.EventsEmit(ctx, "receiving-progress", map[string]string{
				"id":            fileId,
				"file":          msg.Name,
				"totalChunks":   strconv.Itoa(msg.TotalChunks),
				"totalReceived": strconv.Itoa(f.receivedCount),
			})
			p.lastSeen = time.Now()
		}
	}

	//check if all chunks received

	complete := false

	if f.receivedCount == msg.TotalChunks {
		complete = true
	}

	if complete {
		fileName := filepath.Base(msg.Name)

		log.Printf("FileName: %v\n", fileName)

		outFile, _ := os.Create(filepath.Join(downloadPath, fileName))

		for _, chunk := range f.received {
			outFile.Write(chunk)
		}
		outFile.Close()

		for _, fn := range p.fileReceivedListeners {
			fn(fileName)
		}

		log.Printf("Large file received %v\n", fileName)

		time.Sleep(1 * time.Second)

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
	log.Printf("Peer disconnected %v %v\n", p.peer.Name, err)

	if p.onDisconnected != nil {
		p.onDisconnected(p.peer)
	}

	if p.onError != nil && err != io.EOF {
		p.onError(err)
	}
}
