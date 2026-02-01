package transport

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
)

type Hello struct {
	ID       string
	Name     string
	Port     int
	Version  string
	UserName string
}

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

	log.Printf("Remote Version = %v selfVersion = %v\n", remoteHello.Version, peer.Version)
	if remoteHello.Version != peer.Version {
		return fmt.Errorf("version mismatch")
	}

	if conn.role == inbound {
		if err := sendHello(ctx, conn.conn, peer); err != nil {
			return err
		}
	}

	log.Printf("Remote Hello %v", remoteHello)

	/*if options.PermissionFunc != nil {
		//receiver part
		allowed, err := options.PermissionFunc(ctx, discovery.SenderInfo{
			ID:       remoteHello.ID,
			Name:     remoteHello.Name,
			UserName: remoteHello.UserName,
			Files:    remoteHello.Files,
		})

		if err != nil {
			log.Printf("error checking permissions: %v", err)
			return err
		}

		if !allowed {
			log.Printf("Permission denied by user\n")
			permissionResponse := &PermissionResponse{
				Allowed: false,
				Code:    PermDeniedByUser,
				Message: "Permission denied by user",
			}

			sendPermissionResponse(conn.conn, permissionResponse)
		} else {
			log.Printf("Permission granted by user\n")
			permissionResponse := &PermissionResponse{
				Allowed: true,
				Code:    PermOK,
				Message: "Permission granted by user",
			}
			sendPermissionResponse(conn.conn, permissionResponse)
		}
	} else {
		//sender receives permission
		permissionResponse, err := receivePermission(conn.conn)
		if err != nil {
			log.Printf("error receiving permission: %v", err)
			return err
		}
		if !permissionResponse.Allowed {
			log.Printf("Permission denied by user\n")
			return fmt.Errorf("Permission denied by user\n")
		}

		log.Printf("Permission allowed by user\n")
	}

	*/

	conn.peer = discovery.Peer{
		ID:        remoteHello.ID,
		Name:      remoteHello.Name,
		Addresses: conn.peer.Addresses,
		Port:      remoteHello.Port,
		Version:   remoteHello.Version,
	}

	return nil
}

func receivePermission(conn net.Conn) (*PermissionResponse, error) {
	data, err := readFrame(conn)

	if err != nil {
		return nil, err
	}

	var permissionResponse *PermissionResponse
	if err := json.Unmarshal(data, &permissionResponse); err != nil {
		return nil, err
	}

	return permissionResponse, nil
}

func sendHello(ctx context.Context, w io.Writer, peer *discovery.Peer) error {
	log.Printf("Sending Hello %v\n", peer)
	hello := &Hello{
		ID:       peer.ID,
		Name:     peer.Name,
		Port:     peer.Port,
		Version:  peer.Version,
		UserName: peer.UserName,
	}

	log.Printf("Marshalling Hello %v\n", hello)

	data, err := json.Marshal(hello)
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

func sendPermissionResponse(writer io.Writer, permissionResponse *PermissionResponse) {
	data, err := json.Marshal(permissionResponse)
	if err != nil {
		log.Printf("Error while marshalling permission response: %v\n", err)
	}

	if err := writeFrame(writer, data); err != nil {
		log.Printf("Error while writing frame: %v\n", err)
	}
}
