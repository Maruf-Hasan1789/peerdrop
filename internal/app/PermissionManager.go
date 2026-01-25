package app

import (
	"context"
	"sync"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/labstack/gommon/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type PermissionManager struct {
	responses sync.Map
}

func NewPermissionManager() *PermissionManager {
	return &PermissionManager{}
}

func (manager *PermissionManager) Request(ctx context.Context, senderInfo discovery.SenderInfo) (bool, error) {
	log.Info("Request from", senderInfo)
	ch := make(chan bool, 1)
	log.Printf("Permission Manager pointer in Request %p\n", manager)
	manager.responses.Store(senderInfo.ID, ch)

	runtime.EventsEmit(ctx, "permission-request", senderInfo)

	select {
	case allowed := <-ch:
		return allowed, nil
	case <-ctx.Done():
		manager.responses.Delete(senderInfo.ID)
		return false, ctx.Err()
	}
}

func (manager *PermissionManager) Resolve(peerId string, allowed bool) {
	log.Printf("Permission Manager pointer in Resolve %p\n", manager)
	if ch, ok := manager.responses.Load(peerId); ok {
		ch.(chan bool) <- allowed
		manager.responses.Delete(peerId)
	}
}
