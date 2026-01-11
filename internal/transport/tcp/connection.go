package transport

import (
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
)

type connRole int

const (
	inbound connRole = iota
	outbound
)

type Connection interface {
	Send([]byte) error
	Receive() ([]byte, error)
	Close() error
	PeerInfo() discovery.Peer
}

type tcpConnection struct {
	conn net.Conn
	peer discovery.Peer
	mu   sync.Mutex
	role connRole
}

func NewTCPConnection(conn net.Conn, peer discovery.Peer, role connRole) Connection {
	return &tcpConnection{conn: conn, peer: peer, role: role}
}

func (conn *tcpConnection) Send(data []byte) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	length := uint32(len(data))

	//writing length first
	if err := binary.Write(conn.conn, binary.BigEndian, length); err != nil {
		return err
	}

	//writing payload
	_, err := conn.conn.Write(data)

	return err
}

func (conn *tcpConnection) Receive() ([]byte, error) {
	var length uint32

	if err := binary.Read(conn.conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	buf := make([]byte, length)

	_, err := io.ReadFull(conn.conn, buf)

	return buf, err
}

func (conn *tcpConnection) Close() error {
	return conn.conn.Close()
}

func (conn *tcpConnection) PeerInfo() discovery.Peer {
	return conn.peer
}
