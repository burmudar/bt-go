package peer

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
)

const (
	BitTorrentProtocol = "BitTorrent protocol"
	HandshakeLength    = 1 + 19 + 20 + 20 // length + protocol string + hash + peerid
)

type Client struct {
	PeerID string
}

type Handshake struct {
	PeerID string
	Hash   [20]byte
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

func (c *Client) DoHandshake(m *types.FileMeta, p *types.Peer) (*Handshake, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", p.String())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tcp address: %w", err)
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	data, err := encodeHandshake(&Handshake{
		PeerID: c.PeerID,
		Hash:   m.Hash,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding failure: %w", err)
	}

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	fmt.Fprintln(os.Stderr, "writing handshake")
	if _, err = rw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send data: %w", err)
	}
	fmt.Fprintln(os.Stderr, "sending handshake")
	rw.Flush()

	resp := [1024]byte{}
	fmt.Fprintln(os.Stderr, "reading handshake response")
	if _, err := rw.Read(resp[:]); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	fmt.Fprintln(os.Stderr, "decoding handshake response")
	return decodeHandshake(resp[:])
}
