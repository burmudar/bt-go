package peer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type worker struct {
	peerID string
	peer   *types.Peer
	client *Client

	queue <-chan *types.BlockPlan
}

type Pool struct {
	peerID    string
	peers     *types.PeerSpec
	available map[int]*worker
	busy      map[int]*worker

	done <-chan bool

	sync.Mutex
}

func newWorker(peerID string, peer *types.Peer) *worker {
	return &worker{
		peerID: peerID,
		peer:   peer,
		client: NewClient(peerID),
	}
}

func (w *worker) init(torrentHash [20]byte) error {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.client.Connect(ctx, w.peer); err != nil {
		return fmt.Errorf("failed to connect to client: %v", err)
	}

	if _, err := w.client.Handshake(torrentHash); err != nil {
		return fmt.Errorf("failed to perform handshake to client: %v", err)
	}

	return nil
}

func (w *worker) listen(q chan *types.BlockPlan, c chan *types.BlockPlan, quit chan bool) {
	go func() {
		for {
			select {
			case b := <-q:
				{
					_, _ = w.client.DownloadPiece(nil, 0)
					c <- b
				}
			case <-quit:
				return

			}
		}
	}()
}

func NewPool(peerID string, peers *types.PeerSpec) *Pool {
	workers := map[int]*worker{}
	for idx, p := range peers.Peers {
		workers[idx] = newWorker(peerID, p)
	}
	return &Pool{
		peerID:    peerID,
		peers:     peers,
		available: workers,
		done:      make(<-chan bool),
	}
}

func (p *Pool) Init(torrent *types.Torrent) error {
	wg := sync.WaitGroup{}

	hash := torrent.Hash
	errCh := make(chan error)
	for _, w := range p.available {
		wg.Add(1)
		worker := w
		go func() {
			err := worker.init(hash)
			if err != nil {
				errCh <- err
			}
			wg.Done()
		}()
	}
	// gather errors
	errs := []error{}
	go func() {
		for err := range errCh {
			errs = append(errs, err)
		}
	}()
	wg.Wait()
	close(errCh)

	return errors.Join(errs...)
}

func (p *Pool) process(t *types.Torrent, blockSize int, dst string) {
	blocks := t.AllBlockPlans(blockSize)

	queue := make(chan *types.BlockPlan, 5)
	complete := make(chan *types.BlockPlan, 1)
	quit := make(chan bool)
	for _, w := range p.available {
		go w.listen(queue, complete, quit)
	}

	blocksComplete := 0
	inprogress := 0
	blockIdx := 0
	for blocksComplete != len(blocks) {
		select {
		case <-complete:
			{
				blocksComplete++
			}
		default:
			{
				for inprogress != 5 {
					blockIdx++
					queue <- blocks[blockIdx]
					inprogress++
				}
			}
		}
	}
	close(quit)
	close(queue)
	close(complete)
}

func (p *Pool) Download(t *types.Torrent, blockSize int, dst string) {
	go p.process(t, blockSize, dst)

}
