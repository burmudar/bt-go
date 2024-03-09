package peer

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

func NewPool(peerID string, peers *types.PeerSpec) *Pool {
	workers := map[int]*PeerHandler{}
	for idx, p := range peers.Peers {
		workers[idx] = newPeerHandler(idx, peerID, p)
	}
	return &Pool{
		peerID:    peerID,
		peers:     peers,
		available: workers,
		errored:   map[int]*PeerHandler{},
		ready:     map[int]*PeerHandler{},
		done:      make(<-chan bool),
	}
}

func (p *Pool) addPeerWorker(w *PeerHandler) {
	p.Lock()
	if w.Err != nil || !w.handshaked {
		p.errored[w.ID] = w
		delete(p.available, w.ID)
	} else {
		p.ready[w.ID] = w
	}
	defer p.Unlock()
}

func (p *Pool) Init(torrent *types.Torrent) (bool, error) {
	hash := torrent.Hash
	tp := NewTaskPool[any](5)
	for _, w := range p.available {
		worker := w
		tp.Add(&Task[any]{
			Fn: func(_ *reporter) (any, error) {
				worker.Init(hash)
				p.addPeerWorker(worker)
				return nil, worker.Err
			},
		})
	}

	// we use a task pool to start the peer workers concurrently
	tp.Init()
	defer tp.Close()
	err := tp.Process()
	if err != nil {
		return false, err
	}
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	_, err = tp.AwaitComplete(ctx)
	if err != nil {
		return false, err
	}

	return len(p.ready) > 0, nil
}

func (p *Pool) process(t *types.Torrent, blockSize int, dst string) error {
	blocks := t.AllBlockPlans(blockSize)
	fmt.Printf(`Summary:
Peers: %d
Available: %d
Ready: %d
Errored: %d
Pieces: %d
Blocks: %d
`, len(p.peers.Peers), len(p.available), len(p.ready), len(p.errored), len(t.PieceHashes), len(blocks))

	queue := make(chan *types.BlockPlan, 5)
	complete := make(chan *types.PieceDownloadResult, 1)
	quit := make(chan bool)
	for _, w := range p.ready {
		go w.Listen(queue, complete, quit)
	}

	wg := sync.WaitGroup{}
	pieces := []*types.Piece{}

	wg.Add(1)
	go func() {
		left := len(blocks)
		for left > 0 {
			result := <-complete
			if result.Err != nil {
				fmt.Printf("error downloading piece: %v\n", result.Err)
				queue <- result.Plan
			}
			pieces = append(pieces, result.Result)
			fmt.Printf("[%d/%d] complete\n", result.Plan.PieceIndex, len(blocks))
			left--
		}
		wg.Done()
	}()

	for _, blk := range blocks {
		queue <- blk
	}

	wg.Wait()
	close(quit)
	close(queue)
	close(complete)

	sort.Slice(pieces, func(i, j int) bool {
		return pieces[i].Index < pieces[j].Index
	})
	fd, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fd.Close()
	for _, p := range pieces {
		if _, err := fd.Write(p.Data); err != nil {
			return err
		}
	}

	return nil
}

func (p *Pool) Download(t *types.Torrent, blockSize int, dst string) chan bool {
	complete := make(chan bool)
	go func() {
		err := p.process(t, blockSize, dst)
		if err != nil {
			fmt.Printf("pool process failure: %v\n", err)
		}
		complete <- true
	}()

	return complete

}
