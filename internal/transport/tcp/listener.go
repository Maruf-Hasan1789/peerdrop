package transport

import (
	"context"
	"log"
	"net"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
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

	return NewTCPConnection(conn, *selfPeer, inbound), nil
}

func (listener *Listener) GetPort() int {
	if listener.listener == nil {
		return -1
	}

	return listener.listener.Addr().(*net.TCPAddr).Port
}
