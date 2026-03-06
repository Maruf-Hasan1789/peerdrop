package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transport/pb"
	"google.golang.org/protobuf/proto"
)

type PermissionResponse struct {
	Allowed bool
	Code    PermissionCode
	Message string
}

type PermissionCode int

const (
	PermOK PermissionCode = iota
	PermDeniedByUser
	PermPolicyDenied
	PermVersionMismatch
	PermBusy
)

func handshake(ctx context.Context, conn *tcpConnection, peer *discovery.Peer) error {
	log.Printf("Here in handshake\n")
	_ = conn.conn.SetDeadline(time.Now().Add(30 * time.Second))

	defer conn.conn.SetDeadline(time.Time{})

	//send hello
	if conn.role == outbound {
		if err := sendHello(ctx, conn.conn, peer); err != nil {
			return err
		}
	}

	//receive hello
	remoteHello, err := receiveHello(conn.conn)

	log.Printf("Received hello from %v\n", remoteHello)
	if err != nil {
		log.Printf("error receiving hello: %v", err)
		return err
	}

	log.Printf("Remote Version = %v selfVersion = %v\n", remoteHello.GetVersion(), peer.Version)
	if remoteHello.GetVersion() != peer.Version {
		return fmt.Errorf("version mismatch")
	}

	if conn.role == inbound {
		if err := sendHello(ctx, conn.conn, peer); err != nil {
			return err
		}
	}

	log.Printf("Remote Hello %v", remoteHello)

	remotePeer := discovery.Peer{
		ID:      remoteHello.GetId(),
		Name:    remoteHello.GetName(),
		Port:    int(remoteHello.GetPort()),
		Version: remoteHello.GetVersion(),
	}

	conn.peer = remotePeer

	log.Printf("Connected to %v\n", remotePeer)

	log.Printf("Connection peer %v", conn.peer)

	return nil
}

func sendHello(ctx context.Context, w io.Writer, peer *discovery.Peer) error {
	log.Printf("Sending Hello %v\n", peer)
	hello := &pb.Hello{
		Id:       peer.ID,
		Name:     peer.Name,
		Port:     int32(peer.Port),
		Version:  peer.Version,
		UserName: peer.UserName,
	}

	log.Printf("Marshalling Hello %v\n", hello)

	data, err := proto.Marshal(hello)
	if err != nil {
		return err
	}

	return writeFrame(w, data)
}

func receiveHello(r io.Reader) (*pb.Hello, error) {
	data, err := readFrame(r)

	if err != nil {
		return nil, err
	}

	hello := &pb.Hello{}

	if err := proto.Unmarshal(data, hello); err != nil {
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
	log.Printf("Writing Frame\n")
	length := uint32(len(payload))

	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		log.Printf("Error while writing binary write %v\n", err)
		return err
	}

	_, err := w.Write(payload)

	log.Printf("Error while writing payload %v\n", err)
	return err
}
