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
		return nil, err
	}

	tcpConnection := &tcpConnection{
		conn: conn,
		peer: peer,
		role: outbound,
	}

	/*fileInfo, err := os.Stat(actualFilePath)
	if err != nil {
		log.Printf("Error getting file stats %v\n", err)
		return nil, err
	}

	fmt.Println(fileInfo)
	hash := sha256.New()

	file, err := os.Open(actualFilePath)

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file %v\n", err)
		}
	}(file)

	if err != nil {
		log.Printf("Error opening file %v\n", err)
		return nil, err
	}

	if _, err := io.Copy(hash, file); err != nil {
		log.Printf("Error reading file %v\n", err)
		return nil, err
	}

	/*checkSum := hash.Sum(nil)

	/*handshakeOptions := protocol.HandshakeOptions{
		SendOptions: protocol.SendOptions{
			Files: []domain.FileMetadata{
				{
					ID:       strconv.FormatInt(time.Now().UnixMilli(), 10),
					FileName: filepath.Base(actualFilePath),
					FileSize: fileInfo.Size() / 1024,
					Checksum: fmt.Sprintf("%x", checkSum),
				},
			},
		},
	}
	*/
	if err := handshake(ctx, tcpConnection, &peer); err != nil {
		return nil, err
	}

	return tcpConnection, nil
}
