package encoding

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

	r.ReadChar() // populate the first char

	return &r
}

func (b *BencodeReader) ReadInt() (int, error) {
	if b.ch != 'i' {
		return 0, fmt.Errorf("expected i to be current index")
	}
	b.ReadChar()

	num := []byte{}
	for b.ch != 'e' && b.Err == nil {
		num = append(num, b.ch)
		b.ReadChar()
	}
	b.ReadChar() // read past e

	return strconv.Atoi(string(num))
}

func (b *BencodeReader) ReadString() (string, error) {
	num := []byte{}
	for b.ch != ':' && b.Err == nil {
		num = append(num, b.ch)
		b.ReadChar()
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
	n, err := b.input.Read(data)
	if n < length {
		return "", fmt.Errorf("read %d but length to read is %d", n, length)
	}
	// advance
	b.ReadChar()
	return string(data[:n]), err
}

func (b *BencodeReader) ReadChar() {
	if b.input.Len() == 0 {
		b.Err = io.EOF
		return
	}
	b.ch, b.Err = b.input.ReadByte()
}
