package main

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

type BencodeReader struct {
	input *bytes.Reader
	// readPosition points to the next place we will read
	Err  error
	ch   byte
	curr string
}

func NewBencodeReader(input string) *BencodeReader {
	r := BencodeReader{
		input: bytes.NewReader([]byte(input)),
	}

	r.readChar() // populate the first char

	return &r
}

func (b *BencodeReader) readInt() (int, error) {
	if b.ch != 'i' {
		return 0, fmt.Errorf("expected i to be current index")
	}
	b.readChar()

	num := []byte{}
	for b.ch != 'e' && b.Err == nil {
		num = append(num, b.ch)
		b.readChar()
	}
	b.readChar() // read past e

	return strconv.Atoi(string(num))
}

func (b *BencodeReader) readString() (string, error) {
	num := []byte{}
	for b.ch != ':' && b.Err == nil {
		num = append(num, b.ch)
		b.readChar()
	}
	if b.Err != nil {
		return "", b.Err
	}

	length, err := strconv.Atoi(string(num))
	if err != nil {
		return "", err
	}
	// now we can read the string with the length that we got
	data := make([]byte, length)
	_, err = b.input.Read(data)
	// advance
	b.readChar()
	return string(data), err
}

func (b *BencodeReader) readChar() {
	if b.input.Len() == 0 {
		b.Err = io.EOF
		return
	}
	b.ch, b.Err = b.input.ReadByte()
}
