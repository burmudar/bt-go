package peer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

var ErrHandshake = fmt.Errorf("failed to perform handshake")

type worker struct {
	debug      bool
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
		debug:  os.Getenv("DEBUG") == "1",
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
		w.Err = err
		return
	}

	if _, err := w.client.Handshake(torrentHash); err != nil {
		w.Err = ErrHandshake
		return
	}

	w.handshaked = true

}

func (w *worker) announcef(format string, vars ...any) {
	if w.debug {
		fmt.Printf(format, vars...)
	}
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
						w.announcef("worker-%d error: %v\n", w.ID, err)
						break loop
					}
					c <- piece
					w.announcef("worker-%d download of piece %x complete\n", w.ID, blk.Hash)
					break loop
				}
			case <-quit:
				break loop

			}
		}
		w.announcef("worker-%d stopped\n", w.ID)
	}()
}
