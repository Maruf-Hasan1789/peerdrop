package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
)

func main() {
	port := flag.Int("port", 9000, "port")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	selfPeer := discovery.GetSelfPeer(*port)

	listener, err := transport.NewListener(selfPeer)

	if err != nil {
		log.Printf("Error while listening")
	}

	go listenForConnections(listener, ctx, selfPeer)

	log.Printf("Port : %v\n", *port)
	d := discovery.New()

	cliObserver := &discovery.CLIObserver{
		Discovery: d,
	}

	d.AddObserver(cliObserver)

	//start registering
	go func() {
		if err := d.Register(ctx, selfPeer); err != nil {
			log.Fatal(err)
		}
	}()

	//start browsing
	go func() {
		d.Browse(ctx, *selfPeer)
	}()

	time.Sleep(time.Second * 5)

	buf := bufio.NewReader(os.Stdin)
	for {
		//showPeersToUser(d, ctx)
		fmt.Printf("Enter command\n")
		input, _ := buf.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "showPeers":
			showPeersToUser(d, ctx)
		case "exit":
			waitForShutDown()
			cancel()
		default:
			fmt.Println("Unknown Command " + input)
		}
	}
}

func showPeersToUser(d *discovery.Discovery, ctx context.Context) {

	peerList := d.GetPeers()

	fmt.Printf("Peer List size %v\n", len(peerList))

	for _, peer := range peerList {
		fmt.Printf("Peer ID: %v Name = %v\n", peer.ID, peer.Name)
	}

	var choice int
	//userChoiceForPeer:
	fmt.Scan(&choice)

	fmt.Printf("Choice %v\n", choice)

	if choice < 0 || choice >= len(peerList) {
		log.Printf("User choice %v\n", choice)

		//goto userChoiceForPeer
	}

	selectedPeer := peerList[choice]
	dialer := &transport.Dialer{}

	conn, err := dialer.Dial(*selectedPeer, ctx)

	if err != nil {
		log.Printf("Error while creating connection during dialing %v\n", err)
		return
	}

	go handleConnection(conn)
}

func listenForConnections(listener *transport.Listener, ctx context.Context, selfPeer *discovery.Peer) {
	fmt.Printf("Listening for connections\n")
	for {
		conn, err := listener.Accept(ctx, selfPeer)

		if err != nil {
			log.Printf("Error while getting connection from listener %v\n", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn transport.Connection) {
	defer conn.Close()

	done := make(chan struct{})

	go func() {
		readLoop(conn)
		close(done)
	}()
	conn.Send([]byte("Hello Maruf"))

	<-done
}

func readLoop(conn transport.Connection) {
	for {
		msg, err := conn.Receive()

		if err != nil {
			return
		}

		handleMessage(msg)
	}
}

func handleMessage(msg []byte) {
	log.Printf("Message %v\n", string(msg))
}

func waitForShutDown() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
