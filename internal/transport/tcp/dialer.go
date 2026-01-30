package transport

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/domain"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
)

type Dialer struct{}

func (dialer *Dialer) Dial(peer discovery.Peer, ctx context.Context, fileName string) (Connection, error) {

	fmt.Printf("dialed peer %v %v\n", peer.Addresses[0], peer.Port)

	address := fmt.Sprintf("%s:%v", peer.Addresses[0].String(), peer.Port)
	conn, err := net.Dial("tcp", address)

	if err != nil {
		log.Printf("Error occurred while connecting %v\n", err)
		return nil, err
	}

	tcpConnection := &tcpConnection{
		conn: conn,
		peer: peer,
		role: outbound,
	}
	handshakeOptions := protocol.HandshakeOptions{
		SendOptions: protocol.SendOptions{
			Files: []domain.FileMetadata{
				{
					ID:       strconv.FormatInt(time.Now().UnixMilli(), 10),
					FileName: fileName,
					FileSize: int64(0),
					Checksum: fileName,
				},
			},
		},
	}

	if err := handshake(ctx, tcpConnection, &peer, &handshakeOptions); err != nil {
		return nil, err
	}

	return tcpConnection, nil
}
