package transfer

import (
	"context"
	"sync"

	"github.com/labstack/gommon/log"
)

type Registry struct {
	mu       sync.Mutex
	byPeer   map[string][]*TransferTask
	byRootId map[string]*TransferTask
}

func NewRegistry() *Registry {
	return &Registry{
		byPeer:   make(map[string][]*TransferTask),
		byRootId: make(map[string]*TransferTask),
	}
}

func (r *Registry) PauseAllByPeerId(peerId string) {
	log.Printf("Pausing by peer %s", peerId)
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range r.byPeer[peerId] {
		if t.Meta.Status == InProgress {
			t.Meta.Status = Paused
		}
	}
}

func (r *Registry) AddTransferredFile(
	ctx context.Context,
	peerId string,
	transferId,
	rootId string,
	rootName string,
	status Status,
	transferredBytes int64,
	totalBytes int64,
	direction Direction) *TransferTask {
	log.Printf("Add file when receiving \n")
	r.mu.Lock()
	defer r.mu.Unlock()

	registryCtx, cancel := context.WithCancel(ctx)

	if t, ok := r.byRootId[rootId]; ok {
		return t
	}
	t := &Transfer{
		PeerId:           peerId,
		TransferId:       transferId,
		RootID:           rootId,
		RootName:         rootName,
		Status:           status,
		TransferredBytes: transferredBytes,
		TotalBytes:       totalBytes,
		Direction:        direction,
	}

	transferTask := &TransferTask{
		Meta:   t,
		Ctx:    registryCtx,
		Cancel: cancel,
	}

	r.byPeer[peerId] = append(r.byPeer[peerId], transferTask)
	r.byRootId[rootId] = transferTask

	return transferTask
}

func (r *Registry) RemoveFileRegistryUponCompletion(peerId string, rootId string) {
	log.Printf("Remove file receiving upon completion\n")
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byRootId, rootId)
	for i, t := range r.byPeer[peerId] {
		if t.Meta.RootID == rootId {
			r.byPeer[peerId] = append(r.byPeer[peerId][:i], r.byPeer[peerId][i+1:]...)
			break
		}
	}

	if len(r.byPeer[peerId]) == 0 {
		delete(r.byPeer, peerId)
	}
}

func (r *Registry) GetAllTransfersByPeerId(peerId string) []*TransferTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byPeer[peerId]
}

func (r *Registry) UpdateTransferRegistryStatusByRootId(rootId string, status Status) {
	registry := r.byRootId[rootId]
	registry.Meta.Status = status
	log.Printf("RootID %v Registry Status %v\n", r.byRootId[rootId].Meta.RootID, r.byRootId[rootId].Meta.Status)
}

func (r *Registry) GetTransferRegistryByRootId(rootId string) *TransferTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byRootId[rootId]
}
