package peer

import (
	"bufio"
	"bytes"
	"context"
	"testing"
)

func TestDecodeMessage(t *testing.T) {
	tt := []struct {
		name   string
		data   []byte
		wanted Message
	}{
		{
			"decoding Choke",
			[]byte{0, 0, 0, 1, byte(ChokeType)},
			&Choke{},
		},
		{
			"decoding Unchoke",
			[]byte{0, 0, 0, 1, byte(UnchokeType)},
			&Unchoke{},
		},
		{
			"decoding Interested",
			[]byte{0, 0, 0, 1, byte(InterestedType)},
			&Interested{},
		},
		{
			"decoding NotInterested",
			[]byte{0, 0, 0, 1, byte(NotInterestedType)},
			&NotInterested{},
		},
		{
			"decoding Have",
			[]byte{0, 0, 0, 1, byte(HaveType)},
			&Have{},
		},
		{
			"decoding BitField",
			[]byte{0, 0, 0, 1, byte(BitFieldType)},
			&BitField{},
		},
		{
			"decoding Piece Request",
			[]byte{0, 0, 0, 13, byte(RequestType), 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 10},
			&PieceRequest{Index: 1, Begin: 1, Length: 10},
		},
		{
			"decoding Piece Block",
			[]byte{0, 0, 0, 16, byte(PieceType), 0, 0, 0, 1, 0, 0, 0, 1, 119, 105, 108, 108, 105, 97, 109},
			&PieceBlock{Index: 1, Begin: 1, Data: []byte("william")},
		},
		{
			"decoding Cancel",
			[]byte{0, 0, 0, 1, byte(CancelType)},
			&Cancel{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tc.data)
			msg, err := DecodeMessage(context.TODO(), bufio.NewReader(buf))
			if err != nil {
				t.Errorf("error decoding message: %v", err)
			}
			if msg.Tag() != tc.wanted.Tag() {
				t.Errorf("wrong Tag - expected (%d) %T but got (%d) %T", tc.wanted.Tag(), tc.wanted, msg.Tag(), msg)
			}
			if !msg.Equal(tc.wanted) {
				t.Errorf("not equal: expected %#v got %#v", tc.wanted, msg)
			}
		})
	}
}

func TestEncodeMessage(t *testing.T) {
	tt := []struct {
		name    string
		message Message
		wanted  []byte
	}{
		{
			"encode Choke",
			&Choke{},
			[]byte{0, 0, 0, 1, byte(ChokeType)},
		},
		{
			"encode Piece Request",
			&PieceRequest{Index: 1, Begin: 1, Length: 10},
			[]byte{0, 0, 0, 13, byte(RequestType), 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 10},
		},
		{
			"encode Piece Block",
			&PieceBlock{Index: 1, Begin: 1, Data: []byte("william")},
			[]byte{0, 0, 0, 16, byte(PieceType), 0, 0, 0, 1, 0, 0, 0, 1, 119, 105, 108, 108, 105, 97, 109},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			data := EncodeMessage(tc.message)
			if !bytes.Equal(tc.wanted, data) {
				t.Errorf("encoded message did not match expected: want %b got %b", tc.wanted, data)
			}
		})
	}
}

func TestEncodeAndDecodeOfSameMsg(t *testing.T) {
	tt := []struct {
		name    string
		message Message
	}{
		{
			"encode/decode Choke",
			&Choke{},
		},
		{
			"encode/decode Piece Request",
			&PieceRequest{Index: 1, Begin: 1, Length: 10},
		},
		{
			"encode/decode Piece Block",
			&PieceBlock{Index: 1, Begin: 1, Data: []byte("william")},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			data := EncodeMessage(tc.message)
			other, err := DecodeMessage(context.TODO(), bufio.NewReader(bytes.NewBuffer(data)))
			if err != nil {
				t.Fatalf("failed to decode message: %v", err)
			}
			if !other.Equal(tc.message) {
				t.Errorf("message after encode/decode not equal to original: %#v != %#v", other, tc.message)
			}
		})
	}
}
