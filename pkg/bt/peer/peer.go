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

func NewHandshakedClient(ctx context.Context, id string, peer *types.Peer, torrent *types.Torrent) (*Client, error) {
	client := NewClient(id)

	if err := client.Connect(ctx, peer); err != nil {
		return nil, fmt.Errorf("failed to connect to client: %v", err)
	}

	if _, err := client.Handshake(ctx, torrent.Hash); err != nil {
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

func (c *Client) Handshake(ctx context.Context, hash [20]byte) (*Handshake, error) {
	doHandshake := func() (*Handshake, error) {
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

	select {
	case <-ctx.Done():
		{
			return nil, ctx.Err()
		}
	default:
		{
			return doHandshake()
		}
	}
}

func (c *Client) waitForUnchoke() error {
	ticker := time.NewTicker(1 * time.Second)
	done := time.NewTimer(30 * time.Second)

	interested := &Interested{}
	for {
		select {
		case <-ticker.C:
			{
				c.announcef("sending \"interested\"\n")
				data := EncodeMessage(interested)
				if err := c.send(data); err != nil {
					return err
				}
				c.announcef("reading msg")
				if msg, err := DecodeMessage(c.bufrw); err != nil {
					return err
				} else if msg.Tag() != UnchokeType {
					c.announcef("waiting for unchoke - got %T\n", msg)
				} else {
					c.announcef("received unchoke - %T\n", msg)
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

func (c *Client) BitField() (Message, error) {
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
	if err := c.send(data); err != nil {
		return err
	}
	// msg, err := DecodeMessage(c.bufrw)
	// if err != nil {
	// 	return err
	// }
	//
	// c.announcef("(HAVE) recieved message %T", msg)
	return nil
}

func (c *Client) DownloadPiece(plan *types.BlockPlan) (*types.Piece, error) {
	defer c.announcef("<<<< End DownloadPiece [%d] >>>>", plan.PieceIndex)
	if c.Peer == nil {
		panic("cannot download piece with nil peer")
	}
	// 1. bitfield
	// 2. interested
	// 3. unchoke
	// 4. request
	// 5. piece
	if err := c.waitForUnchoke(); err != nil {
		return nil, err
	}

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
			c.announcef("received choke\n")
			if err := c.waitForUnchoke(); err != nil {
				return nil, err
			}
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

	return piece, nil
}
