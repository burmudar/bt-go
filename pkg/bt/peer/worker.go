package peer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

var ErrHandshake = fmt.Errorf("failed to perform handshake")

type worker struct {
	ID         int
	peerID     string
	peer       *types.Peer
	client     *Client
	handshaked bool
	Err        error

	queue <-chan *types.BlockPlan
}

type Pool struct {
	peerID    string
	peers     *types.PeerSpec
	available map[int]*worker
	ready     map[int]*worker
	errored   map[int]*worker

	done <-chan bool

	sync.Mutex
}

func newWorker(id int, peerID string, peer *types.Peer) *worker {
	return &worker{
		ID:     id,
		peerID: peerID,
		peer:   peer,
		client: NewClient(peerID),
	}
}

func (w *worker) Init(torrentHash [20]byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := w.client.Connect(ctx, w.peer); err != nil {
		fmt.Printf("failed to connect to client: %v\n", err)
		w.Err = err
		return
	}

	if _, err := w.client.Handshake(torrentHash); err != nil {
		fmt.Printf("[worker-%d] failed to perform handshake to client: %v\n", w.ID, err)
		w.Err = ErrHandshake
		return
	}

	w.handshaked = true

}

func (w *worker) Listen(q chan *types.BlockPlan, c chan *types.Piece, quit chan bool) {
	go func() {
	loop:
		for {
			select {
			case blk := <-q:
				{
					piece, err := w.client.DownloadPiece(blk)
					if err != nil {
						fmt.Printf("worker-%d error: %v\n", w.ID, err)
						break loop
					}
					c <- piece
					fmt.Printf("worker-%d download of piece %x complete\n", w.ID, blk.Hash)
					break loop
				}
			case <-quit:
				break loop

			}
		}
		fmt.Printf("worker-%d stopped\n", w.ID)
	}()
}
