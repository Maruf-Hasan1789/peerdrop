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
	Send([]byte, []byte) error
	Receive() ([]byte, []byte, error)
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

func (conn *tcpConnection) Send(header []byte, payload []byte) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	headerLen := len(header)
	payloadLen := len(payload)

	totalLen := 4 + headerLen + payloadLen // headerLength + Header + payloadLength (needed for framing)

	var prefix [8]byte
	binary.BigEndian.PutUint32(prefix[:4], uint32(totalLen))
	binary.BigEndian.PutUint32(prefix[4:8], uint32(headerLen))

	buffers := net.Buffers{
		prefix[:],
		header,
		payload,
	}

	_, err := buffers.WriteTo(conn.conn)

	return err
}

func (conn *tcpConnection) Receive() ([]byte, []byte, error) {
	var totalLengthBuf [4]byte

	if _, err := io.ReadFull(conn.conn, totalLengthBuf[:]); err != nil {
		return nil, nil, err
	}

	totalLength := binary.BigEndian.Uint32(totalLengthBuf[:])

	//we can use pre-allocated buffer pool here (need to read about it)
	buf := make([]byte, totalLength)
	if _, err := io.ReadFull(conn.conn, buf); err != nil {
		return nil, nil, err
	}

	headerLen := binary.BigEndian.Uint32(buf[:4])
	header := buf[4 : 4+headerLen]
	payload := buf[4+headerLen:]

	return header, payload, nil
}

func (conn *tcpConnection) Close() error {
	return conn.conn.Close()
}

func (conn *tcpConnection) PeerInfo() discovery.Peer {
	return conn.peer
}
