package peer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
)

const (
	BitTorrentProtocol = "BitTorrent protocol"
	HandshakeLength    = 1 + 19 + 20 + 20 // length + protocol string + hash + peerid

	PieceMsgID      = 7
	BitFieldMsgID   = 5
	InterestedMsgID = 2
	UnchokeMsgID    = 1
)

type Client struct {
	PeerID        string
	Peer          *types.Peer
	conn          net.Conn
	bufrw         *bufio.ReadWriter
	lastHandshake *Handshake
}

type Handshake struct {
	PeerID string
	Hash   [20]byte
}

type PeerMessage struct {
	ID      uint
	Length  uint32
	Payload []byte
}

func decodeBitField(pmsg *PeerMessage) error   { return nil }
func decodeInterested(pmsg *PeerMessage) error { return nil }
func decodeUnchoke(pmsg *PeerMessage) error    { return nil }
func decodePiece(pmsg *PeerMessage) error      { return nil }

func (c *Client) readPeerMessage() (*PeerMessage, error) {
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

	return &PeerMessage{
		ID:      uint(id),
		Length:  length,
		Payload: data,
	}, nil
}

func processPeerMessage(msg *PeerMessage) error {
	switch msg.ID {
	case BitFieldMsgID:
		{
			return decodeBitField(msg)
		}
	case InterestedMsgID:
		{
			return decodeInterested(msg)
		}
	case UnchokeMsgID:
		{
			return decodeUnchoke(msg)
		}
	case PieceMsgID:
		{
			return decodePiece(msg)
		}
	}

	return nil
}

func NewClient(peerID string) (*Client, error) {
	return &Client{
		PeerID: peerID,
	}, nil
}

func encodeHandshake(h *Handshake) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte(byte(19))
	buf.Write([]byte(BitTorrentProtocol))
	buf.Write(bytes.Repeat([]byte{0}, 8))
	buf.Write(h.Hash[:])
	buf.Write([]byte(h.PeerID))

	return buf.Bytes(), nil
}

func decodeHandshake(data []byte) (*Handshake, error) {
	if len(data) < HandshakeLength {
		return nil, fmt.Errorf("malformed handshake - expected length %d got %d", HandshakeLength, len(data))
	}
	buf := bytes.NewBuffer(data)

	b, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("length read failure: %w", err)
	}

	length := int(b)
	if length != 19 {
		return nil, fmt.Errorf("incorrect length - got %d", length)
	}

	part := buf.Next(length)
	proto := string(part)
	if proto != BitTorrentProtocol {
		return nil, fmt.Errorf("incorrect protocol - expected %q got %q", BitTorrentProtocol, proto)
	}

	// skip 8 bytes ahead since that is reserved and we don't care yet about that
	buf.Next(8)

	if buf.Len()+20 > len(data) {
		return nil, fmt.Errorf("not enough data in handshake - cannot read info_hash")
	}
	// read the info_hash
	part = buf.Next(20)
	var hash [20]byte
	copy(hash[:], part[:])

	if buf.Len()+20 > len(data) {
		return nil, fmt.Errorf("not enough data in handshake - cannot read peer id")
	}
	// read the peerID
	part = buf.Next(20)

	return &Handshake{
		PeerID: string(part[:]),
		Hash:   hash,
	}, nil

}

func (c *Client) Handshaked() bool {
	return c.lastHandshake != nil
}

func (c *Client) DownloadPiece(m *types.FileMeta, piece int) error {
	if !c.Handshaked() {
		_, err := c.DoHandshake(m)
		if err != nil {
			return err
		}
	}
	fmt.Println("read bitfield message...")
	// bitfield
	// TODO: think about retries?
	msg, err := c.readPeerMessage()

	fmt.Printf("ID: %d Length: %d Real: %d\n", msg.ID, msg.Length, len(msg.Payload))

	return err
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

func (c *Client) DoHandshake(m *types.FileMeta) (*Handshake, error) {
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
