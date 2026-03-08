package transport

import (
	"context"
	"fmt"
	"net"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/labstack/gommon/log"
)

type Dialer struct{}

func (dialer *Dialer) Dial(ctx context.Context, peer discovery.Peer, selfPeer *discovery.Peer) (Connection, error) {

	//fmt.Printf("dialed peer %v %v\n", peer.Addresses[0], peer.Port)

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

	if err := handshake(ctx, tcpConnection, selfPeer); err != nil {
		return nil, err
	}

	return tcpConnection, nil
}
