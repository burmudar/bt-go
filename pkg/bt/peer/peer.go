package peer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

const (
	BitTorrentProtocol = "BitTorrent protocol"
	HandshakeLength    = 1 + 19 + 20 + 20 // length + protocol string + hash + peerid

)

type Client struct {
	PeerID        string
	Peer          *types.Peer
	conn          net.Conn
	bufrw         *bufio.ReadWriter
	lastHandshake *Handshake
}

func (c *Client) writeMessage(msg Message) error {
	data := EncodeMessage(msg)
	return c.send(data)
}

func NewClient(peerID string) *Client {
	return &Client{
		PeerID: peerID,
	}
}

func NewHandshakedClient(id string, peer *types.Peer, torrent *types.Torrent) (*Client, error) {
	client := NewClient(id)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx, peer); err != nil {
		return nil, fmt.Errorf("failed to connect to client: %v", err)
	}

	if _, err := client.Handshake(torrent.Hash); err != nil {
		return nil, fmt.Errorf("failed to perform handshake to client: %v", err)
	}
	return client, nil
}

func (c *Client) Handshaked() bool {
	return c.lastHandshake != nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Connect(ctx context.Context, p *types.Peer) error {
	fmt.Printf("connecting to: %s\n", p.String())
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", p.String())
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.Peer = p
	c.conn = conn
	c.bufrw = bufio.NewReadWriter(bufio.NewReader(c.conn), bufio.NewWriter(c.conn))

	fmt.Printf("connected to: %s\n", p.String())

	return nil
}

func (c *Client) IsConnected() bool {
	return c.conn != nil
}

func (c *Client) send(data []byte) error {
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("send failure: %w", err)
	}
	return nil
}

func (c *Client) Handshake(hash [20]byte) (*Handshake, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	fmt.Println("starting handshake ...")

	data, err := encodeHandshake(&Handshake{
		PeerID: c.PeerID,
		Hash:   hash,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding failure: %w", err)
	}
	if err := c.send(data); err != nil {
		return nil, err
	}

	resp := [68]byte{}
	if _, err := read(c.bufrw, resp[:]); err != nil {
		return nil, err
	}

	h, err := decodeHandshake(resp[:])
	if err == nil {
		c.lastHandshake = h
	}
	fmt.Println("handshake complete...")

	return h, err
}

func (c *Client) waitForUnchoke() error {
	ticker := time.NewTicker(1 * time.Second)
	done := time.NewTimer(30 * time.Second)

	interested := &Interested{}
	for {
		select {
		case <-ticker.C:
			{
				fmt.Println("sending \"interested\"")
				data := EncodeMessage(interested)
				if err := c.send(data); err != nil {
					return err
				}
				fmt.Println("reading msg")
				if msg, err := DecodeMessage(c.bufrw); err != nil {
					return err
				} else if msg.Tag() != UnchokeType {
					fmt.Printf("waiting for unchoke - got %T\n", msg)
				} else {
					fmt.Printf("received unchoke - %T\n", msg)
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

func assembleData(blocks []*PieceBlock) []byte {
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Begin < blocks[j].Begin
	})

	// The last block may be smaller than the regular chunk size
	result := make([]byte, 0)
	for _, block := range blocks {
		result = append(result, block.Data...)
	}

	return result
}

func (c *Client) BitField() (Message, error) {
	fmt.Println("read bitfield message...")
	msg, err := DecodeMessage(c.bufrw)
	if err != nil {
		return nil, err
	}

	if msg.Tag() != BitFieldType {
		return nil, fmt.Errorf("expected BitField msg but got ID %d", msg.Tag())
	}

	return msg, nil
}

func (c *Client) DownloadPiece(t *types.Torrent, pIndex int) (*types.Piece, error) {
	// 1. bitfield
	// 2. interested
	// 3. unchoke
	// 4. request
	// 5. piece
	if err := c.waitForUnchoke(); err != nil {
		return nil, err
	}

	plan := t.BlockPlan(pIndex, 16*1024)
	downloaded := make([]*PieceBlock, plan.NumBlocks)
	for i := 0; i < plan.NumBlocks; i++ {
		req := &PieceRequest{
			Index:  pIndex,
			Begin:  i * plan.BlockSize,
			Length: plan.BlockSizeFor(i),
		}

		fmt.Printf("requesting %d - Begin: %d Length: %d\n", req.Index, req.Begin, req.Length)
		data := EncodeMessage(req)
		if err := c.send(data); err != nil {
			return nil, err
		}
		msg, err := DecodeMessage(c.bufrw)
		if err != nil {
			return nil, err
		}
		switch m := msg.(type) {
		case *KeepAlive:
			fmt.Println("received keep alive")
		case *Choke:
			fmt.Println("received choke")
			if err := c.waitForUnchoke(); err != nil {
				return nil, err
			}
		case *PieceBlock:
			{
				fmt.Printf("received block %d for piece %d - Begin: %d Length: %d\n", i, m.Index, m.Begin, len(m.Data))
				downloaded[i] = m
			}
		}
	}

	data := assembleData(downloaded)
	piece := &types.Piece{
		Index: pIndex,
		Peer:  *c.Peer,
		Size:  t.LengthOf(pIndex),
		Data:  data,
	}

	return piece, nil
}
