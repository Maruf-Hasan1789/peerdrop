package transport

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
)

type Hello struct {
	ID      string
	Name    string
	Port    int
	Version string
}

func handshake(ctx context.Context, conn *tcpConnection, self *discovery.Peer) error {

	_ = conn.conn.SetDeadline(time.Now().Add(5 * time.Second))

	defer conn.conn.SetDeadline(time.Time{})

	//send hello
	if conn.role == outbound {
		if err := sendHello(conn.conn, self); err != nil {
			return err
		}
	}

	//receive hello
	remoteHello, err := receiveHello(conn.conn)

	if err != nil {
		return err
	}

	if conn.role == inbound {
		if err := sendHello(conn.conn, self); err != nil {
			return err
		}
	}

	log.Printf("Remote Version = %v selfVersion = %v\n", remoteHello.Version, self.Version)
	if remoteHello.Version != self.Version {
		return fmt.Errorf("version mismatch")
	}

	conn.peer = discovery.Peer{
		ID:        remoteHello.ID,
		Name:      remoteHello.Name,
		Addresses: conn.peer.Addresses,
		Port:      remoteHello.Port,
		Version:   remoteHello.Version,
	}

	return nil
}

func sendHello(w io.Writer, peer *discovery.Peer) error {
	data, err := json.Marshal(Hello{
		ID:      peer.ID,
		Name:    peer.Name,
		Port:    peer.Port,
		Version: peer.Version,
	})

	if err != nil {
		return err
	}

	return writeFrame(w, data)
}

func receiveHello(r io.Reader) (*Hello, error) {
	data, err := readFrame(r)

	if err != nil {
		return nil, err
	}

	var hello *Hello

	if err := json.Unmarshal(data, &hello); err != nil {
		return nil, err
	}

	return hello, nil
}

func readFrame(r io.Reader) ([]byte, error) {
	var length uint32

	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	buf := make([]byte, length)

	_, err := io.ReadFull(r, buf)
	return buf, err
}

func writeFrame(w io.Writer, payload []byte) error {
	length := uint32(len(payload))

	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		log.Printf("Error while writing binary write %v\n", err)
		return err
	}

	_, err := w.Write(payload)

	log.Printf("Error while writing payload %v\n", err)
	return err
}
