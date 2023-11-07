package peer

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	KeepAliveType     MessageTag = 99
	ChokeType         MessageTag = 0
	UnchokeType       MessageTag = 1
	InterestedType    MessageTag = 2
	NotInterestedType MessageTag = 3
	HaveType          MessageTag = 4
	BitFieldType      MessageTag = 5
	RequestType       MessageTag = 6
	PieceType         MessageTag = 7
	CancelType        MessageTag = 8
)

type MessageTag int

var ErrNotImplemented = fmt.Errorf("not implemented")
var ErrUnknownMessage = fmt.Errorf("unknown message")

type Handshake struct {
	PeerID string
	Hash   [20]byte
}

type Message interface {
	Tag() MessageTag
	ToRaw() *RawMessage
}

type RawMessage struct {
	Tag     uint
	Length  uint32
	Payload []byte
}

func (raw *RawMessage) Bytes() []byte {
	data := make([]byte, 4+1+len(raw.Payload))

	binary.BigEndian.PutUint32(data, raw.Length)
	data[4] = byte(raw.Tag)
	if len(raw.Payload) > 0 {
		copy(data[5:], raw.Payload)
	}
	return data
}

type KeepAlive struct{}

func (k *KeepAlive) Tag() MessageTag { return KeepAliveType }
func (k *KeepAlive) ToRaw() *RawMessage {
	return &RawMessage{
		Tag:    uint(k.Tag()),
		Length: 0,
	}
}

type Choke struct{}

func (c *Choke) Tag() MessageTag { return ChokeType }
func (c *Choke) ToRaw() *RawMessage {
	return &RawMessage{
		Tag:    uint(c.Tag()),
		Length: 5,
	}
}

type Unchoke struct{}

func (u *Unchoke) Tag() MessageTag { return UnchokeType }
func (u *Unchoke) ToRaw() *RawMessage {
	return &RawMessage{
		Tag:    uint(u.Tag()),
		Length: 1,
	}
}

type Interested struct{}

func (i *Interested) Tag() MessageTag { return InterestedType }
func (i *Interested) ToRaw() *RawMessage {
	return &RawMessage{
		Tag:    uint(i.Tag()),
		Length: 1,
	}
}

type NotInterested struct{}

func (n *NotInterested) Tag() MessageTag { return NotInterestedType }
func (n *NotInterested) ToRaw() *RawMessage {
	return &RawMessage{
		Tag:    uint(n.Tag()),
		Length: 1,
	}
}

type Have struct{}

func (h *Have) Tag() MessageTag    { return HaveType }
func (h *Have) ToRaw() *RawMessage { return nil }

type BitField struct {
	Field []byte
}

func (b *BitField) Tag() MessageTag    { return BitFieldType }
func (b *BitField) ToRaw() *RawMessage { return nil }

type PieceRequest struct {
	// Index is the zero index of the piece
	Index int
	// Begin is the zero based offset of with in the piece
	Begin int
	// Length is the length of the block in bytes
	Length int
}

func (r *PieceRequest) Tag() MessageTag { return RequestType }
func (r *PieceRequest) ToRaw() *RawMessage {
	data := make([]byte, 12)
	binary.BigEndian.PutUint32(data[0:4], uint32(r.Index))
	binary.BigEndian.PutUint32(data[4:8], uint32(r.Begin))
	binary.BigEndian.PutUint32(data[8:12], uint32(r.Length))
	return &RawMessage{
		Tag:     uint(r.Tag()),
		Length:  uint32(len(data)),
		Payload: data,
	}
}

type PieceBlock struct {
	// Index is the zero index of the piece
	Index int
	// Begin is the zero based offset of with in the piece
	Begin int
	// Length is the length of the block in bytes
	Data []byte
}

func (p *PieceBlock) Tag() MessageTag    { return PieceType }
func (p *PieceBlock) ToRaw() *RawMessage { return nil }

type Cancel struct{}

func (c *Cancel) Tag() MessageTag    { return CancelType }
func (c *Cancel) HasPayload() bool   { return true }
func (p *Cancel) ToRaw() *RawMessage { return nil }

func decodeBitField(msg *RawMessage) (*BitField, error) {
	var result BitField
	result.Field = msg.Payload
	return &result, nil
}

func decodeRequest(msg *RawMessage) (*PieceRequest, error) { return nil, nil }

func decodePiece(msg *RawMessage) (*PieceBlock, error) {
	var block PieceBlock

	block.Index = int(binary.BigEndian.Uint32(msg.Payload[0:4])) // 4 bytes
	block.Begin = int(binary.BigEndian.Uint32(msg.Payload[4:8])) // 4 bytes
	//blkLen := msg.Length - 8
	block.Data = msg.Payload[8:]

	return &block, nil
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

func encodeHandshake(h *Handshake) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte(byte(19))
	buf.Write([]byte(BitTorrentProtocol))
	buf.Write(bytes.Repeat([]byte{0}, 8))
	buf.Write(h.Hash[:])
	buf.Write([]byte(h.PeerID))

	return buf.Bytes(), nil
}

func decodeMessage(msg *RawMessage) (Message, error) {
	switch MessageTag(msg.Tag) {
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
	case KeepAliveType:
		{
			return &KeepAlive{}, nil
		}
	}

	fmt.Printf("TAG: %d\n", msg.Tag)
	return nil, ErrUnknownMessage
}
