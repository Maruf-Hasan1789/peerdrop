package discovery

import "fmt"

type PeerObserver interface {
	OnPeerAdded(peer Peer)
	OnPeerRemoved(peer Peer)
}

type CLIObserver struct {
	Discovery *Discovery
}

func (cliObserver *CLIObserver) OnPeerAdded(peer Peer) {
	fmt.Printf("New Peer ID = %v PeerName = %v Peer Port = %v, Version = %v\n", peer.ID, peer.Name, peer.Port, peer.Version)

	peers := cliObserver.Discovery.GetPeers()
	fmt.Printf("Active Peers:\n")

	for _, peer := range peers {
		fmt.Printf("Peer ID = %v Name = %v Port = %v Version = %v\n", peer.ID, peer.Name, peer.Port, peer.Version)
	}
}

func (cliObserver *CLIObserver) OnPeerRemoved(peer Peer) {

}
