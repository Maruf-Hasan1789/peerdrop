package transfer

import (
	"sync"
)

type Registry struct {
	mu       sync.Mutex
	byPeer   map[string][]*Transfer
	byRootId map[string]*Transfer
}

func NewRegistry() *Registry {
	return &Registry{
		byPeer:   make(map[string][]*Transfer),
		byRootId: make(map[string]*Transfer),
	}
}

func (r *Registry) PauseAllByPeerId(peerId string) {
	//log.Printf("Pausing by peer %s", peerId)
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range r.byPeer[peerId] {
		if t.Status == InProgress {
			t.Status = Paused
		}
	}
}

func (r *Registry) AddTransferredFile(peerId string, transferId, rootId string, rootName string, status Status, transferredBytes int64, totalBytes int64, direction Direction) *Transfer {
	//log.Printf("Add file when receiving \n")
	r.mu.Lock()
	defer r.mu.Unlock()

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

	r.byPeer[peerId] = append(r.byPeer[peerId], t)
	r.byRootId[rootId] = t

	return t
}

func (r *Registry) RemoveFileRegistryUponCompletion(peerId string, rootId string) {
	//log.Printf("Remove file receiving upon completion\n")
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byRootId, rootId)
	for i, t := range r.byPeer[peerId] {
		if t.RootID == rootId {
			r.byPeer[peerId] = append(r.byPeer[peerId][:i], r.byPeer[peerId][i+1:]...)
			break
		}
	}

	if len(r.byPeer[peerId]) == 0 {
		delete(r.byPeer, peerId)
	}
}

func (r *Registry) GetAllTransfersByPeerId(peerId string) []*Transfer {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byPeer[peerId]
}

func (r *Registry) UpdateTransferRegistryStatusByRootId(rootId string, status Status) {
	registry := r.byRootId[rootId]
	registry.Status = status
	//log.Printf("RootID %v Registry Status %v\n", r.byRootId[rootId].RootID, r.byRootId[rootId].Status)
}

func (r *Registry) GetTransferRegistryByRootId(rootId string) *Transfer {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byRootId[rootId]
}
