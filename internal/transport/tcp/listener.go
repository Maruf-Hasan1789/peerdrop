package transport

import (
	"context"
	"net"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/labstack/gommon/log"
)

type Listener struct {
	listener net.Listener
}

func NewListener() (*Listener, error) {
	listener, err := net.Listen("tcp", ":0")

	if err != nil {
		log.Printf("Couldn't listen to tcp %v\n", err.Error())
		return nil, err
	}

	return &Listener{listener: listener}, nil
}

func (listener *Listener) Accept(ctx context.Context, selfPeer *discovery.Peer) (Connection, error) {
	conn, err := listener.listener.Accept()

	if err != nil {
		log.Printf("Error while creating connection in listener %v\n", err)

		return nil, err
	}

	tcpConnection := &tcpConnection{
		conn: conn,
		role: inbound,
	}

	err = handshake(ctx, tcpConnection, selfPeer)

	if err != nil {
		log.Printf("Error while handshake from %v\n", err)
		err = tcpConnection.Close()
		return nil, err
	}

	log.Printf("Connection PeerInfo %v", tcpConnection.PeerInfo())

	return NewTCPConnection(conn, tcpConnection.peer, inbound), nil
}

func (listener *Listener) GetPort() int {
	if listener.listener == nil {
		return -1
	}

	return listener.listener.Addr().(*net.TCPAddr).Port
}
