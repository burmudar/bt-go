package peer

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

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
	fmt.Printf("type: %d bytes: %x\n", msg.Tag, data)
	return c.send(data)
}

func (c *Client) readMessage() (Message, error) {
	raw, err := c.recvMessage()
	if err != nil {
		return nil, err
	}
	return decodeMessage(raw)
}

func (c *Client) recvMessage() (*RawMessage, error) {
	prefix := make([]byte, 5)
	if _, err := c.recv(prefix); err != nil {
		return nil, err
	}

	fmt.Printf("Received bytes %x\n", prefix)

	length := binary.BigEndian.Uint32(prefix[0:4])
	if length == 0 {
		return &RawMessage{
			Tag:     uint(KeepAliveType),
			Length:  length,
			Payload: nil,
		}, nil
	}

	tag := uint(prefix[4])
	fmt.Printf("Message Tag: %d Len: %d\n", tag, length)
	msg := &RawMessage{
		Tag:     tag,
		Length:  length,
		Payload: nil,
	}

	if length > 1 {
		msg.Payload = make([]byte, length-1) // -1  because we don't want the message tag
		// we probably want to chunk recv this
		if _, err := c.recv(msg.Payload); err != nil {
			return nil, err
		}
	}

	return msg, nil
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

func (c *Client) recv(data []byte) (int, error) {
	fmt.Printf("receiving %d buffered %d\n", len(data), c.bufrw.Reader.Buffered())
	if len(data) == 0 {
		return 0, nil
	}
	if n, err := c.bufrw.Read(data[:]); err != nil {
		return n, fmt.Errorf("receive failure: %w", err)
	} else {
		return n, nil
	}
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
	if _, err := c.recv(resp[:]); err != nil {
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
				if err := c.writeMessage(interested.ToRaw()); err != nil {
					return err
				}
				fmt.Println("reading msg")
				if msg, err := c.readMessage(); err != nil {
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
	msg, err := c.readMessage()
	if err != nil {
		return err
	}

	if msg.Tag() != BitFieldType {
		return fmt.Errorf("expected BitField msg but got ID %d", msg.Tag())
	}

	if err := c.waitForUnchoke(); err != nil {
		return err
	}

	chunkSize := 16 * 1024
	blocks := make([][]byte, m.Length/chunkSize)

	fmt.Printf("need to request %d blocks for piece %d\n", len(blocks), pIndex)

	req := PieceRequest{
		Index:  0,
		Begin:  0,
		Length: chunkSize,
	}

	fmt.Printf("requesting %d - Begin: %d Length: %d\n", req.Index, req.Begin, req.Length)
	c.writeMessage(req.ToRaw())

	for {
		msg, err := c.readMessage()
		if err != nil {
			return err
		}
		switch m := msg.(type) {
		case *KeepAlive:
			fmt.Println("received keep alive")
		case *Choke:
			fmt.Println("received choke")
			if err := c.waitForUnchoke(); err != nil {
				return err
			}
		case *PieceBlock:
			{
				fmt.Printf("Block Index:%d Begin:%d Data Len:%d\n", m.Index, m.Begin, len(m.Data))
			}

		}
	}

}
