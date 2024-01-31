package peer

import (
	"fmt"
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type PeerPieceRegistration struct {
	worker *PeerHandler
	pieces []int
}

type PeerManager struct {
	pending []*PeerHandler

	peerRegistrar  *registrar[int, *PeerHandler]
	pieceRegistrar *registrar[int, int]

	initPool *TaskPool
	taskPool *TaskPool

	initOnce sync.Once
}

func NewPeerManager(peerID string, peers []*types.Peer) *PeerManager {
	m := PeerManager{}
	m.pending = []*PeerHandler{}
	for i, p := range peers {
		m.pending = append(m.pending, newPeerHandler(i, peerID, p))
	}

	m.peerRegistrar = newRegistrar[int, *PeerHandler]()
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

		go m.peerRegistrar.listen()
		go m.pieceRegistrar.listen()

		tasks := []*Task{}
		for i, w := range m.pending {
			handler := w
			tasks = append(tasks, &Task{
				ID: i,
				Fn: func(r *reporter) error {
					err := handler.Init(hash)
					if err != nil {
						return err
					}
					m.peerRegistrar.C <- registration[int, *PeerHandler]{
						Key:   handler.ID,
						Value: handler,
					}

					pieces := handler.QueryPieces()
					for _, p := range pieces {
						m.pieceRegistrar.C <- registration[int, int]{
							Key:   p,
							Value: handler.ID,
						}
					}

					r.L("%d peers registered\n", m.peerRegistrar.Len())
					r.L("%d pieces registered\n", m.pieceRegistrar.Len())

					return nil
				},
			})
		}

		fmt.Printf("initPool: process %d tasks\n", len(tasks))
		<-m.initPool.Process(tasks)
	})

	return err
}
