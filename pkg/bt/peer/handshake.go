package peer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
)

const HandshakeType MessageTag = 98

func (h *Handshake) Equal(m Message) bool {
	other, ok := m.(*Handshake)
	if !ok {
		return false
	}

	return bytes.Equal(h.Hash[:], other.Hash[:])
}
func (h *Handshake) Tag() MessageTag { return HandshakeType }
func (h *Handshake) String() string  { return "Handshake" }
func (h *Handshake) Payload() []byte {
	var buf bytes.Buffer

	buf.WriteByte(byte(19))
	buf.Write([]byte(BitTorrentProtocol))
	buf.Write(bytes.Repeat([]byte{0}, 8))
	buf.Write(h.Hash[:])
	buf.Write([]byte(h.PeerID))

	return buf.Bytes()
}

func doHandshake(ctx context.Context, conn net.Conn, peerID string, hash [20]byte) (*Handshake, error) {
	println("writing handshake")
	us, err := writeHandshake(conn, peerID, hash)
	if err != nil {
		return nil, err
	}
	println("reading handshake")
	them, err := readHandshake(conn)
	if err != nil {
		return nil, err
	}

	println("comparing handshakes")

	if !us.Equal(them) {
		return nil, fmt.Errorf("handshake mismatch\nsent: %x\nreceived: %x", us.Hash, them.Hash)
	}

	return them, nil
}

func writeHandshake(conn net.Conn, peerID string, hash [20]byte) (*Handshake, error) {
	handshake := &Handshake{
		PeerID: peerID,
		Hash:   hash,
	}

	w := bufio.NewWriter(conn)
	_, err := w.Write(handshake.Payload())
	if err != nil {
		return nil, fmt.Errorf("handshake failure: %v", err)
	}

	return handshake, w.Flush()
}

func readHandshake(conn net.Conn) (*Handshake, error) {
	resp := [68]byte{}
	r := bufio.NewReader(conn)
	if _, err := read(r, resp[:]); err != nil {
		return nil, err
	}

	return decodeHandshake(resp[:])
}
