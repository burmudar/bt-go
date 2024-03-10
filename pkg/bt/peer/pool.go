package peer

import (
	"context"
	"fmt"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
	"github.com/jackc/puddle"
)

type peerPool struct {
	peerID string
	peers  *types.PeerSpec
	pool   *puddle.Pool
}

type Pool interface {
	Get(ctx context.Context) (*Client, func(), error)
}

func NewPool(peerID string, peers *types.PeerSpec, torrent *types.Torrent) (Pool, error) {
	peerQueue := types.NewSyncQueue[*types.Peer]()
	peerQueue.AddAll(peers.Peers...)

	var ctor puddle.Constructor = func(ctx context.Context) (any, error) {
		peer, ok := peerQueue.Pop()
		if !ok {
			return nil, fmt.Errorf("not peers left to construct")
		}

		return NewHandshakedClient(peerID, peer, torrent)
	}

	var dtor puddle.Destructor = func(res interface{}) {
		if client, ok := res.(*Client); ok {
			client.Close()
		}
	}

	return &peerPool{
		peerID: peerID,
		peers:  peers,
		pool:   puddle.NewPool(ctor, dtor, int32(len(peers.Peers))),
	}, nil
}

func (p *peerPool) Get(ctx context.Context) (*Client, func(), error) {
	res, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}

	client, ok := res.Value().(*Client)
	if !ok {
		return nil, nil, fmt.Errorf("expected *peer.Client but got %T", res.Value())
	}

	return client, res.Release, nil
}
