package peer

import (
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
	"github.com/jackc/puddle"
)

type Pool struct {
	peerID string
	peers  *types.PeerSpec
	puddle.Pool
}

func toSet(peers *types.PeerSpec) types.Set[string] {
	s := types.NewSyncSet[string]()
	for _, p := range peers.Peers {
		key := p.String()
		s.Put(key)
	}

	return s
}

func NewPool(peerID string, peers *types.PeerSpec) *Pool {
	peerSet := types.NewSet[string]()

	return &Pool{
		peerID: peerID,
		peers:  peers,
	}
}
