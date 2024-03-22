package peer

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

const (
	BitTorrentProtocol = "BitTorrent protocol"
	HandshakeLength    = 1 + 19 + 20 + 20 // length + protocol string + hash + peerid

)

type Client struct {
	PeerID string
	Peer   *types.Peer

	Channel *Channel
}

type Result[T any] struct {
	R   T
	Err error
}

func NewClient(ctx context.Context, peerID string, peer *types.Peer, hash [20]byte) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	channel, err := NewHandshakedChannel(ctx, peerID, peer, hash)

	if err != nil {
		return nil, err
	}
	return &Client{
		PeerID:  peerID,
		Peer:    peer,
		Channel: channel,
	}, nil
}

func resultWithContext[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	done := make(chan Result[T])
	go func() {
		r, err := fn()
		done <- Result[T]{
			R:   r,
			Err: err,
		}
	}()
	select {
	case <-ctx.Done():
		{
			var empty T
			return empty, ctx.Err()
		}
	case result := <-done:
		{
			return result.R, result.Err
		}
	}
}

func assembleData(blocks []*PieceBlock) ([]byte, error) {
	var sortErr error
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i] == nil {
			sortErr = fmt.Errorf("blocks at i: %d were nil", i)
			return false
		}
		if blocks[j] == nil {
			sortErr = fmt.Errorf("blocks at j: %d were nil", j)
			return false
		}
		return blocks[i].Begin < blocks[j].Begin
	})

	if sortErr != nil {
		return nil, sortErr
	}

	// The last block may be smaller than the regular chunk size
	data := make([]byte, 0)
	for _, block := range blocks {
		data = append(data, block.Data...)
	}

	return data, nil
}

func (c *Client) DownloadPiece(plan *types.BlockPlan) (*types.Piece, error) {
	// 1. bitfield
	// 2. interested
	// 3. unchoke
	// 4. request
	// 5. piece
	c.Channel.SendInterested()
	c.Channel.SendUnchoke()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	err := c.Channel.WaitFor(ctx, BitFieldType)
	cancel()
	if err != nil {
		return nil, err
	}

	if c.Channel.HasPiece(plan.PieceIndex) {
		return nil, ErrPieceUnavailable
	}

	downloaded := make([]*PieceBlock, 0, plan.NumBlocks)

	result := make(chan *PieceBlock, plan.NumBlocks)

	c.Channel.RegisterReceiveHook(PieceType, func(msg Message) error {
		blk, ok := msg.(*PieceBlock)

		if !ok {
			fmt.Printf("expected pieceblock but got %T", msg)
		}

		result <- blk
		return nil
	})

	for i := 0; i < plan.NumBlocks; i++ {
		c.Channel.SendPieceRequest(plan.PieceIndex, i*plan.BlockSize, plan.BlockSizeFor(i))
	}

	for blk := range result {
		fmt.Printf("[%s] receive Piece %d from peer (%d/%d)\n", c.Peer.String(), plan.PieceIndex, len(downloaded), plan.NumBlocks)
		downloaded = append(downloaded, blk)

		if len(downloaded) == plan.NumBlocks {
			break
		}
	}

	c.Channel.SendHave(plan.PieceIndex)
	c.Channel.RemoveReceiveHook(PieceType)
	close(result)

	data, err := assembleData(downloaded)
	if err != nil {
		return nil, err
	}
	piece := &types.Piece{
		Index: plan.PieceIndex,
		Peer:  *c.Peer,
		Size:  plan.PieceLength,
		Data:  data,
		Hash:  sha1.Sum(data),
	}

	c.Channel.SetPiece(piece.Index)

	return piece, nil
}

func (c *Client) Close() {
	c.Channel.Close()
}
