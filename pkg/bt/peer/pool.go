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

func NewPool(peerID string, peers *types.PeerSpec) *Pool {
	constructor :=
	return &Pool{
		peerID: peerID,
		peers:  peers,
	}
}
