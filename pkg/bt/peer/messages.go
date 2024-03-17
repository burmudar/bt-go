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

type MessageTag uint8

var ErrNotImplemented = fmt.Errorf("not implemented")
var ErrUnknownMessage = fmt.Errorf("unknown message")

type Handshake struct {
	PeerID string
	Hash   [20]byte
}

type Message interface {
	Tag() MessageTag
	Equal(m Message) bool
	Payload() []byte
	String() string
}

type RawMessage struct {
	Tag     uint
	Length  uint32
	Payload []byte
}

type KeepAlive struct{}
type Choke struct{}
type Unchoke struct{}
type Interested struct{}
type NotInterested struct{}
type Have struct {
	Index int
}
type BitField struct {
	Field []byte
}

type PieceRequest struct {
	// Index is the zero index of the piece
	Index int
	// Begin is the zero based offset of with in the piece
	Begin int
	// Length is the length of the block in bytes
	Length int
}

type PieceBlock struct {
	// Index is the zero index of the piece
	Index int
	// Begin is the zero based offset of with in the piece
	Begin int
	// Length is the length of the block in bytes
	Data []byte
}
type Cancel struct{}

func (k *KeepAlive) Equal(m Message) bool {
	_, ok := m.(*KeepAlive)
	return ok
}
func (k *KeepAlive) Tag() MessageTag { return KeepAliveType }
func (k *KeepAlive) Payload() []byte {
	return nil
}
func (k *KeepAlive) String() string {
	return "KeepAlive"
}

func (c *Choke) Equal(m Message) bool {
	_, ok := m.(*Choke)
	return ok
}
func (c *Choke) Tag() MessageTag { return ChokeType }
func (c *Choke) String() string  { return "Choke" }
func (c *Choke) Payload() []byte {
	return nil
}

func (u *Unchoke) Equal(m Message) bool {
	_, ok := m.(*Unchoke)
	return ok
}
func (u *Unchoke) Tag() MessageTag { return UnchokeType }
func (u *Unchoke) String() string  { return "Unchoke" }
func (u *Unchoke) Payload() []byte {
	return nil
}

func (i *Interested) Equal(m Message) bool {
	_, ok := m.(*Interested)
	return ok
}
func (i *Interested) Tag() MessageTag { return InterestedType }
func (i *Interested) String() string  { return "Interested" }
func (i *Interested) Payload() []byte {
	return nil
}

func (n *NotInterested) Equal(m Message) bool {
	_, ok := m.(*NotInterested)
	return ok
}
func (n *NotInterested) Tag() MessageTag { return NotInterestedType }
func (n *NotInterested) String() string  { return "NotInterested" }
func (n *NotInterested) Payload() []byte {
	return nil
}

func (h *Have) Equal(m Message) bool {
	_, ok := m.(*Have)
	return ok
}
func (h *Have) Tag() MessageTag { return HaveType }
func (h *Have) String() string  { return "Have" }
func (h *Have) Payload() []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data[0:4], uint32(h.Index))
	return data
}

func (b *BitField) Equal(m Message) bool {
	v, ok := m.(*BitField)
	return ok && bytes.Equal(b.Field, v.Field)
}
func (b *BitField) Tag() MessageTag { return BitFieldType }
func (b *BitField) String() string  { return "BitField" }
func (b *BitField) Payload() []byte { return nil }

func (r *PieceRequest) Equal(m Message) bool {
	v, ok := m.(*PieceRequest)
	if !ok {
		return false
	}

	return v.Index == r.Index && v.Begin == r.Begin && v.Length == r.Length
}
func (r *PieceRequest) Tag() MessageTag { return RequestType }
func (r *PieceRequest) String() string  { return "PieceRequest" }
func (r *PieceRequest) Payload() []byte {
	data := make([]byte, 12)
	binary.BigEndian.PutUint32(data[0:4], uint32(r.Index))
	binary.BigEndian.PutUint32(data[4:8], uint32(r.Begin))
	binary.BigEndian.PutUint32(data[8:12], uint32(r.Length))
	return data
}

func (p *PieceBlock) Equal(m Message) bool {
	v, ok := m.(*PieceBlock)
	if !ok {
		return false
	}

	return v.Index == p.Index && v.Begin == p.Begin && bytes.Equal(v.Data, p.Data)
}
func (p *PieceBlock) Tag() MessageTag { return PieceType }
func (p *PieceBlock) String() string  { return "PieceBlock" }
func (p *PieceBlock) Payload() []byte {
	data := make([]byte, 8+len(p.Data))
	binary.BigEndian.PutUint32(data[0:4], uint32(p.Index))
	binary.BigEndian.PutUint32(data[4:8], uint32(p.Begin))
	copy(data[8:], p.Data)
	return data
}

func (c *Cancel) Equal(m Message) bool {
	_, ok := m.(*Cancel)
	return ok
}
func (c *Cancel) Tag() MessageTag { return CancelType }
func (c *Cancel) String() string  { return "Cancel" }
func (c *Cancel) Payload() []byte { return nil }
