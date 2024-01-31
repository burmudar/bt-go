package peer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

var ErrHandshake = fmt.Errorf("failed to perform handshake")
var ErrPieceHashMismatch = fmt.Errorf("hash mismatch")
var ErrChocked = fmt.Errorf("Peer is choked")

type PeerHandler struct {
	debug      bool
	ID         int
	peerID     string
	peer       *types.Peer
	client     *Client
	peerPieces Set[int]
	handshaked bool
	Err        error
}

type Pool struct {
	peerID    string
	peers     *types.PeerSpec
	available map[int]*PeerHandler
	ready     map[int]*PeerHandler
	errored   map[int]*PeerHandler

	done <-chan bool

	sync.Mutex
}

func newPeerHandler(id int, peerID string, peer *types.Peer) *PeerHandler {
	return &PeerHandler{
		debug:      os.Getenv("DEBUG") == "1",
		ID:         id,
		peerID:     peerID,
		peer:       peer,
		client:     NewClient(peerID),
		peerPieces: NewSet[int](),
	}
}

func (p *PeerHandler) Init(torrentHash [20]byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := p.client.Connect(ctx, p.peer); err != nil {
		return err
	}

	if _, err := p.client.Handshake(torrentHash); err != nil {
		return err
	}

	if msg, err := p.client.ReadBitField(); err != nil {
		p.announcef("bitfield read error: %v\n", err)
	} else {
		p.updatePeerPiecesFromBitField(msg.Payload())
	}

	p.handshaked = true
	return nil

}

func (p *PeerHandler) waitForUnchoke() error {
	ticker := time.NewTicker(1 * time.Second)
	done := time.NewTimer(30 * time.Second)

	for {
		select {
		case <-ticker.C:
			{
				p.announcef("sending \"interested\"\n")
				p.client.Interested()
				p.announcef("reading msg")
				if msg, err := p.client.ReadMsg(); err != nil {
					return err
				} else if msg.Tag() != UnchokeType {
					p.announcef("waiting for unchoke - got %T\n", msg)
				} else {
					p.announcef("received unchoke - %T\n", msg)
					ticker.Stop()
					done.Stop()
					return nil
				}
			}
		case <-done.C:
			{
				ticker.Stop()
				done.Stop()
				return fmt.Errorf("failed to unchoke after 30 seconds")
			}
		}
	}
}

func (p *PeerHandler) updatePeerPiecesFromBitField(field []byte) {
	for i, val := range field {
		// check each bit in val
		for j := 0; j < 8; j++ {
			if val&(1<<(7-j)) != 0 {
				idx := i*8 + j
				p.peerPieces.Put(idx)
			}
		}
	}

	p.announcef("peer reports %d pieces from BitField\n", p.peerPieces.Len())
}

func (p *PeerHandler) announcef(format string, vars ...any) {
	if p.debug {
		fmt.Printf("[worker-%d] ", p.ID)
		fmt.Printf(format, vars...)
	}
}

// TODO: accept context
func (p *PeerHandler) DownloadPiece(blk *types.BlockPlan) *types.PieceDownloadResult {
	result := types.PieceDownloadResult{Plan: blk}
	piece, err := p.client.DownloadPiece(blk)
	if err != nil {
		result.Err = err
		return &result
	}
	if !bytes.Equal(piece.Hash[:], blk.Hash) {
		result.Err = ErrPieceHashMismatch
		return &result
	}
	result.Result = piece

	p.client.Have(blk.PieceIndex)

	return &result
}

func (p *PeerHandler) QueryPieces() []int {
	return p.peerPieces.All()
}

func (p *PeerHandler) Listen(q chan *types.BlockPlan, c chan *types.PieceDownloadResult, quit chan bool) {
	go func() {
	loop:
		for {
			select {
			case blk := <-q:
				{
					c <- p.DownloadPiece(blk)
					p.announcef("processed piece %d\n", blk.PieceIndex)
				}
			case <-quit:
				break loop

			}
		}
		p.announcef("stopped\n", p.ID)
	}()
}
