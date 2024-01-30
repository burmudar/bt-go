package peer

import (
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type PeerPieceRegistration struct {
	worker *PeerWorker
	pieces []int
}

type PeerManager struct {
	pending []*PeerWorker

	peerRegistrar  *registrar[int, *PeerWorker]
	pieceRegistrar *registrar[int, int]

	initPool *TaskPool
	taskPool *TaskPool

	initOnce sync.Once
}

func NewPeerManager(peerID string, peers []*types.Peer) *PeerManager {
	m := PeerManager{}
	m.pending = []*PeerWorker{}
	for i, p := range peers {
		m.pending = append(m.pending, newPeerWorker(i, peerID, p))
	}

	m.peerRegistrar = newRegistrar[int, *PeerWorker]()
	m.pieceRegistrar = newRegistrar[int, int]()

	m.initPool = NewTaskPool(5)
	m.taskPool = NewTaskPool(5)

	return &m
}

func (m *PeerManager) Init(hash [20]byte) error {
	var err error
	m.initOnce.Do(func() {
		m.initPool.Init()
		m.taskPool.Init()

		m.peerRegistrar.listen()
		m.pieceRegistrar.listen()

		tasks := []*Task{}
		for _, w := range m.pending {
			worker := w
			tasks = append(tasks, &Task{
				Fn: func(r *reporter) error {
					err := worker.Init(hash)
					if err != nil {
						return err
					}
					m.peerRegistrar.C <- registration[int, *PeerWorker]{
						Key:   worker.ID,
						Value: worker,
					}

					pieces, err := worker.QueryPieces()
					for _, p := range pieces {
						m.pieceRegistrar.C <- registration[int, int]{
							Key:   p,
							Value: worker.ID,
						}
					}

					return nil
				},
			})
		}
	})

	return err
}
