package protocol

import (
	"context"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/domain"
)

type PermissionFunc func(
	ctx context.Context,
	senderInfo discovery.SenderInfo,
) (bool, error)

type HandshakeOptions struct {
	PermissionFunc PermissionFunc
	SendOptions    SendOptions
}

type SendOptions struct {
	Files []domain.FileMetadata
}
