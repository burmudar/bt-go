package peer

import (
	"context"
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

	initPool     *TaskPool[any]
	downloadPool *TaskPool[*types.Piece]

	initOnce sync.Once
}

func NewPeerManager(peerID string, peers []*types.Peer) *PeerManager {
	m := PeerManager{}
	m.pending = []*PeerHandler{}
	m.initPool = NewTaskPool[any](5)
	m.downloadPool = NewTaskPool[*types.Piece](5)
	for i, p := range peers {
		m.pending = append(m.pending, newPeerHandler(i, peerID, p))
	}

	m.peerRegistrar = newRegistrar[int, *PeerHandler]()
	m.pieceRegistrar = newRegistrar[int, int]()

	return &m
}

func (m *PeerManager) Init(hash [20]byte) error {
	var err error
	m.initOnce.Do(func() {
		m.initPool.Init()
		m.downloadPool.Init()

		tasks := []*Task[any]{}
		for i, w := range m.pending {
			peerHandler := w
			m.initPool.Add(&Task[any]{
				ID: i,
				Fn: func(r *reporter) (any, error) {
					err := peerHandler.Init(hash)
					if err != nil {
						return nil, err
					}
					m.peerRegistrar.Add(peerHandler.ID, peerHandler)
					fmt.Println("peer registered", peerHandler.ID)

					pieces := peerHandler.QueryPieces()
					for _, p := range pieces {
						m.pieceRegistrar.Add(p, peerHandler.ID)
					}

					r.L("%d peers registered\n", m.peerRegistrar.Len())
					r.L("%d pieces registered\n", m.pieceRegistrar.Len())

					return nil, nil
				},
			})
		}

		fmt.Printf("initPool: process %d tasks\n", len(tasks))
		err := m.initPool.Process()
		if err != nil {
			fmt.Printf("initPool: error processing tasks - %v", err)
		}

		m.initPool.AwaitComplete(context.Background())
	})

	return err
}

func (m *PeerManager) Download(pieces []*types.BlockPlan) ([]*types.Piece, error) {
	for i, p := range pieces {
		idx := i
		m.downloadPool.Add(&Task[*types.Piece]{
			ID: i,
			Fn: func(r *reporter) (*types.Piece, error) {
				peer, ok := m.peerRegistrar.Get(idx)
				if !ok {
					return nil, fmt.Errorf("peer %d not found\n", idx)
				}

				r.c <- fmt.Sprintf("downloading piece %d\n", p.PieceIndex)
				res := peer[0].DownloadPiece(p)
				if res.Err != nil {
					return nil, res.Err
				}
				r.c <- fmt.Sprintf("piece %d downloaded\n", p.PieceIndex)
				return res.Result, nil
			},
		})
	}

	if err := m.downloadPool.Process(); err != nil {
		return nil, err
	}

	return m.downloadPool.AwaitComplete(context.Background())
}
