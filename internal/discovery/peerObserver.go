package discovery

type PeerObserver interface {
	OnPeerAdded(peer Peer)
	OnPeerRemoved(peer Peer)
}
