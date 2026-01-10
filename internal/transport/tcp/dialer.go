package transport

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
)

type Dialer struct{}

func (dialer *Dialer) Dial(peer discovery.Peer, ctx context.Context) (Connection, error) {

	fmt.Printf("dialed peer %v %v\n", peer.Addresses[0], peer.Port)

	address := fmt.Sprintf("%s:%v", peer.Addresses[0].String(), peer.Port)
	conn, err := net.Dial("tcp", address)

	if err != nil {
		log.Printf("Error occurred while connecting %v\n", err)
	}

	tcpConnection := &tcpConnection{
		conn: conn,
		peer: peer,
		role: outbound,
	}

	if err := handshake(ctx, tcpConnection, &peer); err != nil {
		return nil, err
	}

	return tcpConnection, nil
}
