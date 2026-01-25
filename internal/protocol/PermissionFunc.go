package protocol

import (
	"context"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
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
	FileNames []string
}
