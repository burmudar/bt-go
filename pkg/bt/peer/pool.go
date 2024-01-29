package peer

import (
	"fmt"
	"os"
	"sort"
	"time"

	"go.uber.org/multierr"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

func NewPool(peerID string, peers *types.PeerSpec) *Pool {
	workers := map[int]*worker{}
	for idx, p := range peers.Peers {
		workers[idx] = newWorker(idx, peerID, p)
	}
	return &Pool{
		peerID:    peerID,
		peers:     peers,
		available: workers,
		errored:   map[int]*worker{},
		ready:     map[int]*worker{},
		done:      make(<-chan bool),
	}
}

func (p *Pool) addPeerWorker(w *worker) {
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
	tasks := []*Task{}
	for _, w := range p.available {
		worker := w
		tasks = append(tasks, &Task{
			Fn: func(_ *reporter) error {
				worker.Init(hash)
				p.addPeerWorker(worker)
				return worker.Err
			},
		})
	}

	// we use a task pool to start the peer workers concurrently
	tp := NewTaskPool(5)
	tp.Init()
	errC := tp.Process(tasks)
	go func() {
		errs := <-errC
		fmt.Printf("some errors during peer start: %v\n", multierr.Combine(errs...))
	}()
	<-time.After(15 * time.Second)
	tp.Close()

	return len(p.ready) > 0, nil //errors.Join(errs...)
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
	complete := make(chan *types.Piece, 1)
	quit := make(chan bool)
	for _, w := range p.ready {
		go w.Listen(queue, complete, quit)
	}

	for _, blk := range blocks {
		queue <- blk
	}

	pieces := []*types.Piece{}
	for r := range complete {
		pieces = append(pieces, r)
		fmt.Printf("%d/%d complete", r.Index, len(t.PieceHashes))
	}
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
