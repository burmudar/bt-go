package peer

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
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

func (c *Client) writeMessage(msg *RawMessage) error {
	data := msg.Bytes()
	return c.send(data)
}

func (c *Client) readMessage() (*RawMessage, error) {
	data := make([]byte, 5)
	if err := c.recv(data); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(data[:4])
	id := int(data[4])
	data = make([]byte, length)
	// we probably want to chunk recv this
	if err := c.recv(data); err != nil {
		return nil, err
	}

	return &RawMessage{
		ID:      uint(id),
		Length:  length,
		Payload: data,
	}, nil
}

func NewClient(peerID string) (*Client, error) {
	return &Client{
		PeerID: peerID,
	}, nil
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
	c.bufrw = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	fmt.Printf("connected to: %s\n", p.String())

	return nil
}

func (c *Client) IsConnected() bool {
	return c.conn != nil
}

func (c *Client) send(data []byte) error {
	if _, err := c.bufrw.Write(data); err != nil {
		return fmt.Errorf("send failure: %w", err)
	}
	c.bufrw.Flush()
	return nil
}

func (c *Client) recv(data []byte) error {
	if _, err := c.bufrw.Read(data[:]); err != nil {
		return fmt.Errorf("receive failure: %w", err)
	}

	return nil
}

func (c *Client) DoHandshake(m *types.Torrent) (*Handshake, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	fmt.Println("starting handshake ...")

	data, err := encodeHandshake(&Handshake{
		PeerID: c.PeerID,
		Hash:   m.Hash,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding failure: %w", err)
	}
	if err := c.send(data); err != nil {
		return nil, err
	}

	resp := [1024]byte{}
	if err := c.recv(resp[:]); err != nil {
		return nil, err
	}

	h, err := decodeHandshake(resp[:])
	if err == nil {
		c.lastHandshake = h
	}
	fmt.Println("handshake complete...")

	return h, err
}

func (c *Client) DownloadPiece(m *types.Torrent, pIndex int) error {
	if !c.Handshaked() {
		_, err := c.DoHandshake(m)
		if err != nil {
			return err
		}
	}
	// 1. bitfield
	// 2. interested
	// 3. unchoke
	// 4. request
	// 5. piece
	fmt.Println("read bitfield message...")
	// bitfield
	// TODO: think about retries?
	raw, err := c.readMessage()
	if err != nil {
		return err
	}
	msg, err := decodeMessage(raw)
	if err != nil {
		return err
	}

	if msg.Type() != BitFieldType {
		return fmt.Errorf("expected BitField msg but got ID %d", msg.Type())
	}

	c.writeMessage((&Interested{}).ToRaw())

	raw, err = c.readMessage()
	if err != nil {
		return err
	}
	msg, err = decodeMessage(raw)
	if err != nil {
		return err
	}
	if msg.Type() != UnchokeType {
		return fmt.Errorf("expected Unchoke msg but got ID %d", msg.Type())
	}

	chunkSize := 16 * 1024
	blocks := make([][]byte, m.Length/chunkSize)

	for i := 0; i < len(blocks); i++ {
		req := PieceRequest{
			Index:  i,
			Begin:  i * chunkSize,
			Length: chunkSize,
		}

		fmt.Printf("Requesting %d - Begin: %d Length: %d\n", req.Index, req.Begin, req.Length)
		c.writeMessage(req.ToRaw())

		retries := 3
		for retries > 0 {
			raw, err = c.readMessage()
			if err != nil {
				return err
			}
			msg, err = decodeMessage(raw)
			if err != nil {
				return err
			}
			if msg.Type() != PieceType {
				fmt.Printf("expected PieceType msg but got ID %d\n", msg.Type())
			}
			retries--
		}

	}

	return err
}
