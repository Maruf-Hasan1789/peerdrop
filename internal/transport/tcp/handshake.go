package transport

import (
	"context"
	"fmt"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transport/pb"
	"github.com/labstack/gommon/log"
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
		if err := sendHello(ctx, conn, peer); err != nil {
			return err
		}
	}

	//receive hello
	remoteHello, err := receiveHello(conn)

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
		if err := sendHello(ctx, conn, peer); err != nil {
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

func sendHello(ctx context.Context, conn Connection, peer *discovery.Peer) error {
	log.Printf("Sending Hello %v\n", peer)
	hello := &pb.Hello{
		Id:       peer.ID,
		Name:     peer.Name,
		Port:     int32(peer.Port),
		Version:  peer.Version,
		UserName: peer.UserName,
	}

	log.Printf("Marshalling Hello %v\n", hello)
	msg := &pb.Message{
		Payload: &pb.Message_Hello{
			Hello: hello,
		},
	}

	data, err := proto.Marshal(msg)

	if err != nil {
		return err
	}

	return conn.Send(data, nil)
}

func receiveHello(conn Connection) (*pb.Hello, error) {
	header, _, err := conn.Receive()

	if err != nil {
		log.Printf("Error while receiving from connection %v\n", err)
		return nil, err
	}

	msg := &pb.Message{}

	if err = proto.Unmarshal(header, msg); err != nil {
		log.Printf("Error while unmarshalling Header in Hello\n")
		return nil, err
	}

	hello := msg.GetHello()

	return hello, nil
}
