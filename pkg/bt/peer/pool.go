package peer

import (
	"context"
	"fmt"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
	"github.com/jackc/puddle"
)

type Pool struct {
	peerID string
	peers  *types.PeerSpec
	puddle.Pool
}

func NewPool(peerID string, peers *types.PeerSpec) *Pool {
	peerQueue := types.NewSyncQueue[*types.Peer]()
	peerQueue.AddAll(peers.Peers...)

	var ctor puddle.Constructor = func(ctx context.Context) (interface{}, error) {
		_, ok := peerQueue.Pop()
		if !ok {
			return nil, fmt.Errorf("not peers left to construct")
		}

		// TODO: user peer

		return nil, nil
	}

	return &Pool{
		peerID: peerID,
		peers:  peers,
	}
}
