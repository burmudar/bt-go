package peer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func read(reader *bufio.ReadWriter, data []byte) (n int, err error) {
	size := len(data)
	start := 0
	for size > 0 {
		if n, err := reader.Read(data[start:]); err != nil {
			if errors.Is(err, io.EOF) {
				return size, err
			} else {
				return n, fmt.Errorf("read failure: %w", err)
			}
		} else {
			start = n
			size -= n
		}
	}

	return size, nil
}

func DecodeRawMessage(r *bufio.ReadWriter) (*RawMessage, error) {
	prefix := make([]byte, 5)
	if _, err := read(r, prefix); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(prefix[0:4])
	if length == 0 {
		return &RawMessage{
			Tag:     uint(KeepAliveType),
			Length:  length,
			Payload: nil,
		}, nil
	}

	tag := uint(prefix[4])
	msg := &RawMessage{
		Tag:     tag,
		Length:  length,
		Payload: nil,
	}

	if length > 1 {
		msg.Payload = make([]byte, length-1) // -1  because we don't want the message tag
		if n, err := read(r, msg.Payload); err != nil {
			if errors.Is(err, io.EOF) {
				println("EOF reached while reading payload - read", n)
			} else {
				return nil, err
			}
		}
	}

	return msg, nil
}

func decodeBitField(msg *RawMessage) (*BitField, error) {
	var result BitField
	result.Field = msg.Payload
	return &result, nil
}

func decodePieceRequest(msg *RawMessage) (*PieceRequest, error) {
	var req PieceRequest

	if len(msg.Payload) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}
	req.Index = int(binary.BigEndian.Uint32(msg.Payload[0:4]))   // 4 bytes
	req.Begin = int(binary.BigEndian.Uint32(msg.Payload[4:8]))   // 4 bytes
	req.Length = int(binary.BigEndian.Uint32(msg.Payload[8:12])) // 4 bytes

	return &req, nil
}

func decodePiece(msg *RawMessage) (*PieceBlock, error) {
	var block PieceBlock

	if len(msg.Payload) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}
	block.Index = int(binary.BigEndian.Uint32(msg.Payload[0:4])) // 4 bytes
	block.Begin = int(binary.BigEndian.Uint32(msg.Payload[4:8])) // 4 bytes

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

func DecodeMessage(r *bufio.ReadWriter) (Message, error) {
	msg, err := DecodeRawMessage(r)
	if err != nil {
		return nil, err
	}
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
			return &Have{}, nil
		}
	case BitFieldType:
		{
			return decodeBitField(msg)
		}
	case RequestType:
		{
			return decodePieceRequest(msg)
		}
	case PieceType:
		{
			return decodePiece(msg)
		}
	case CancelType:
		{
			return &Cancel{}, nil
		}
	case KeepAliveType:
		{
			return &KeepAlive{}, nil
		}
	}

	fmt.Printf("TAG: %d\n", msg.Tag)
	return nil, ErrUnknownMessage
}
