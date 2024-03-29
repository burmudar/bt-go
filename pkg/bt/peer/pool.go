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

		fmt.Printf("\n\n[%s] constructing peer client\n\n", peer.String())
		client, err := NewClient(ctx, peerID, peer, torrent)
		if err != nil {
			fmt.Printf("failed to create handshaked client(%s): %v(%T)\n", peer.String(), err, err)
			peerQueue.Add(peer)
			return nil, err
		}
		return client, err
	}

	var dtor puddle.Destructor = func(res interface{}) {
		if res == nil {
			return
		}
		if client, ok := res.(*Client); ok {
			fmt.Println("destroying - ", client.Peer.String())
			client.Close()
			peerQueue.Add(client.Peer)
		}
	}

	return &peerPool{
		peerID: peerID,
		peers:  peers,
		pool:   puddle.NewPool(ctor, dtor, int32(len(peers.Peers))),
	}, nil
}

func (p *peerPool) Get(ctx context.Context) (*Client, func(), error) {
	noop := func() {}
	res, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, noop, err
	}

	client, ok := res.Value().(*Client)
	if !ok {
		res.Destroy()
		return nil, noop, fmt.Errorf("expected *peer.Client but got %T", res.Value())
	}

	if !client.Channel.IsValid() {
		res.Destroy()
		return nil, noop, fmt.Errorf("[%s] client is invalid - destroying", client.Peer.String())
	}

	release := func() {
		if !client.Channel.IsValid() {
			fmt.Printf("(pool) client[%s] is invalid - destroying\n", client.Peer.String())
			res.Destroy()
		} else {
			res.Release()
		}
	}

	return client, release, nil
}
