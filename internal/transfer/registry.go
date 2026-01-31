package transfer

import (
	"sync"

	"github.com/labstack/gommon/log"
)

type Registry struct {
	mu       sync.Mutex
	byPeer   map[string][]*Transfer
	byFileId map[string]*Transfer
}

func NewRegistry() *Registry {
	return &Registry{
		byPeer:   make(map[string][]*Transfer),
		byFileId: make(map[string]*Transfer),
	}
}

func (r *Registry) PauseAllByPeerId(peerId string) {
	log.Printf("Pausing by peer %s", peerId)
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range r.byPeer[peerId] {
		if t.Status == InProgress {
			t.Status = Paused
		}
	}
}

func (r *Registry) AddFileReceiving(peerId string, fileId string, status Status, chunkDone int64) *Transfer {
	log.Printf("Add file when receiving \n")
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.byFileId[fileId]; ok {
		return t
	}
	t := &Transfer{
		FileId:    fileId,
		PeerId:    peerId,
		Status:    status,
		ChunkDone: chunkDone,
	}

	r.byPeer[peerId] = append(r.byPeer[peerId], t)
	r.byFileId[fileId] = t

	return t
}

func (r *Registry) RemoveFileReceivingUponCompletion(peerId string, fileId string) {
	log.Printf("Remove file receiving upon completion\n")
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byFileId, fileId)
	for i, t := range r.byPeer[peerId] {
		if t.FileId == fileId {
			r.byPeer[peerId] = append(r.byPeer[peerId][:i], r.byPeer[peerId][i+1:]...)
			break
		}
	}

	if len(r.byPeer[peerId]) == 0 {
		delete(r.byPeer, peerId)
	}
}
