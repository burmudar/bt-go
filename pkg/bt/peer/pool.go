package peer

import (
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type Pool struct {
	peers *types.PeerSpec

	sync.Mutex
}

func NewPool(peers *types.PeerSpec) *Pool {
	return &Pool{
		peers: peers,
	}
}
