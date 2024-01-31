package peer

import (
	"bufio"
	"context"
	"crypto/sha1"
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

const (
	BitTorrentProtocol = "BitTorrent protocol"
	HandshakeLength    = 1 + 19 + 20 + 20 // length + protocol string + hash + peerid

)

type Client struct {
	debug         bool
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
		debug:  os.Getenv("DEBUG") == "1",
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
	c.announcef("connecting to: %s\n", p.String())
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", p.String())
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.Peer = p
	c.conn = conn
	c.bufrw = bufio.NewReadWriter(bufio.NewReader(c.conn), bufio.NewWriter(c.conn))

	c.announcef("connected to: %s\n", p.String())

	return nil
}

func (c *Client) IsConnected() bool {
	return c.conn != nil
}

func (c *Client) send(data []byte) error {
	c.announcef("sending %d\n", len(data))
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("send failure: %w", err)
	}
	return nil
}

func (c *Client) Handshake(hash [20]byte) (*Handshake, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	c.announcef("starting handshake ...\n")

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
	c.announcef("handshake complete...\n")

	return h, err
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

func (c *Client) Interested() error {
	data := EncodeMessage(&Interested{})
	if err := c.send(data); err != nil {
		return err
	}
	return nil
}

func (c *Client) ReadMsg() (Message, error) {
	msg, err := DecodeMessage(c.bufrw)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Client) ReadBitField() (Message, error) {
	c.announcef("read bitfield message...\n")
	msg, err := DecodeMessage(c.bufrw)
	if err != nil {
		return nil, err
	}

	if msg.Tag() != BitFieldType {
		return nil, fmt.Errorf("expected BitField msg but got ID %d", msg.Tag())
	}

	return msg, nil
}

func (c *Client) announcef(format string, vars ...any) {
	if c.debug {
		peer := "unknown"
		if c.Peer != nil {
			peer = c.Peer.IP.String()
		}
		fmt.Printf("[%s] ", peer)
		fmt.Printf(format, vars...)
	}
}

func (c *Client) Have(index int) error {
	req := &Have{Index: index}
	c.announcef("Have %d index\n", index)
	data := EncodeMessage(req)
	return c.send(data)
}

func (c *Client) DownloadPiece(plan *types.BlockPlan) (*types.Piece, error) {
	if c.Peer == nil {
		panic("cannot download piece with nil peer")
	}
	// 1. bitfield
	// 2. interested
	// 3. unchoke
	// 4. request
	// 5. piece

	downloaded := make([]*PieceBlock, plan.NumBlocks)
	c.announcef("need to get %d blocks for piece %d\n", plan.NumBlocks, plan.PieceIndex)
	for i := 0; i < plan.NumBlocks; i++ {
		req := &PieceRequest{
			Index:  plan.PieceIndex,
			Begin:  i * plan.BlockSize,
			Length: plan.BlockSizeFor(i),
		}

		c.announcef("requesting %d - Begin: %d Length: %d\n", req.Index, req.Begin, req.Length)
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
			c.announcef("received keep alive\n")
		case *Choke:
			return nil, ErrChocked
		case *PieceBlock:
			{
				c.announcef("received block %d for piece %d - Begin: %d Length: %d\n", i, m.Index, m.Begin, len(m.Data))
				downloaded[i] = m
			}
		default:
			{
				c.announcef("unknown msg received: %+v\n", msg)
			}
		}
	}

	c.announcef("fetched %d blocks for piece %d\n", plan.NumBlocks, plan.PieceIndex)

	data := assembleData(downloaded)
	piece := &types.Piece{
		Index: plan.PieceIndex,
		Peer:  *c.Peer,
		Size:  plan.PieceLength,
		Data:  data,
		Hash:  sha1.Sum(data),
	}

	return piece, nil
}
