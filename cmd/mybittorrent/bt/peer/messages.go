package peer

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	ChokeType         MessageType = 0
	UnchokeType       MessageType = 1
	InterestedType    MessageType = 2
	NotInterestedType MessageType = 3
	HaveType          MessageType = 4
	BitFieldType      MessageType = 5
	RequestType       MessageType = 6
	PieceType         MessageType = 7
	CancelType        MessageType = 7
)

type MessageType int

var ErrNotImplemented = fmt.Errorf("not implemented")
var ErrUnknownMessage = fmt.Errorf("unknown message")

type Handshake struct {
	PeerID string
	Hash   [20]byte
}

type Message interface {
	Type() MessageType
	ToRaw() *RawMessage
}

type RawMessage struct {
	ID      uint
	Length  uint32
	Payload []byte
}

func (raw *RawMessage) Bytes() []byte {
	data := make([]byte, raw.Length)

	binary.BigEndian.PutUint32(data, uint32(len(data)))
	data[4] = byte(raw.ID)
	if len(raw.Payload) > 0 {
		copy(data[4:], raw.Payload)
	}
	return data
}

type Choke struct{}

func (c *Choke) Type() MessageType { return ChokeType }
func (c *Choke) ToRaw() *RawMessage {
	return &RawMessage{
		ID:     uint(c.Type()),
		Length: 5,
	}
}

type Unchoke struct{}

func (u *Unchoke) Type() MessageType { return UnchokeType }
func (u *Unchoke) ToRaw() *RawMessage {
	return &RawMessage{
		ID:     uint(u.Type()),
		Length: 5,
	}
}

type Interested struct{}

func (i *Interested) Type() MessageType { return InterestedType }
func (i *Interested) ToRaw() *RawMessage {
	return &RawMessage{
		ID:     uint(i.Type()),
		Length: 5,
	}
}

type NotInterested struct{}

func (n *NotInterested) Type() MessageType { return NotInterestedType }
func (n *NotInterested) ToRaw() *RawMessage {
	return &RawMessage{
		ID:     uint(n.Type()),
		Length: 5,
	}
}

type Have struct{}

func (h *Have) Type() MessageType  { return HaveType }
func (h *Have) ToRaw() *RawMessage { return nil }

type BitField struct {
	Field []byte
}

func (b *BitField) Type() MessageType  { return BitFieldType }
func (b *BitField) ToRaw() *RawMessage { return nil }

type PieceRequest struct {
	// Index is the zero index of the piece
	Index int
	// Begin is the zero based offset of with in the piece
	Begin int
	// Length is the length of the block in bytes
	Length int
}

func (r *PieceRequest) Type() MessageType { return RequestType }
func (r *PieceRequest) ToRaw() *RawMessage {
	data := []byte{}
	binary.BigEndian.PutUint32(data, uint32(r.Index))
	binary.BigEndian.PutUint32(data, uint32(r.Begin))
	binary.BigEndian.PutUint32(data, uint32(r.Length))
	return &RawMessage{
		ID:      uint(r.Type()),
		Length:  uint32(len(data) + 1),
		Payload: data,
	}
}

type PieceBlock struct{}

func (p *PieceBlock) Type() MessageType  { return PieceType }
func (p *PieceBlock) ToRaw() *RawMessage { return nil }

type Cancel struct{}

func (c *Cancel) Type() MessageType  { return CancelType }
func (c *Cancel) HasPayload() bool   { return true }
func (p *Cancel) ToRaw() *RawMessage { return nil }

func decodeBitField(msg *RawMessage) (*BitField, error) {
	var result BitField
	result.Field = msg.Payload
	return &result, nil
}

func decodeRequest(msg *RawMessage) (*PieceRequest, error) { return nil, nil }
func decodePiece(msg *RawMessage) (*PieceBlock, error)     { return nil, nil }

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

func encodeHandshake(h *Handshake) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte(byte(19))
	buf.Write([]byte(BitTorrentProtocol))
	buf.Write(bytes.Repeat([]byte{0}, 8))
	buf.Write(h.Hash[:])
	buf.Write([]byte(h.PeerID))

	return buf.Bytes(), nil
}

func encodeMessage(msg Message) ([]byte, error) {
	var encoded []byte
	switch msg.(type) {
	case *Choke:
		{
		}
	case *Unchoke:
		{
		}
	case *Interested:
		{
			return nil, nil
		}
	case *NotInterested:
		{
			return nil, nil
		}
	case *Have:
		{
			return nil, ErrNotImplemented
		}
	case *BitField:
		{
			return nil, nil
		}
	case *PieceRequest:
		{
			return nil, ErrNotImplemented
		}
	case *PieceBlock:
		{
			return nil, nil
		}
	}

	return encoded, nil
}

func decodeMessage(msg *RawMessage) (Message, error) {
	switch MessageType(msg.ID) {
	case ChokeType:
		{
			return &Choke{}, nil
		}
	case UnchokeType:
		{
			return &Unchoke{}, nil
		}
	case InterestedType:
		{
			return &Interested{}, nil
		}
	case NotInterestedType:
		{
			return &NotInterested{}, nil
		}
	case HaveType:
		{
			return nil, ErrNotImplemented
		}
	case BitFieldType:
		{
			return decodeBitField(msg)
		}
	case RequestType:
		{
			return nil, ErrNotImplemented
		}
	case PieceType:
		{
			return decodePiece(msg)
		}
	}

	return nil, ErrUnknownMessage
}
