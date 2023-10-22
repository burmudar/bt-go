package main

const (
	EOI             = 0
	NULL_TERMINATOR = '\x00'
)

type BencodeReader struct {
	input    string
	position int
	// readPosition points to the next place we will read
	readPosition int
	ch           byte
	curr         string
}

func NewBencodeReader(input string) *BencodeReader {
	r := BencodeReader{
		input:    input,
		position: -1,
	}

	r.readChar()
	return &r
}

func (b *BencodeReader) readNull() {
	b.ch = b.input[b.readPosition]
	b.curr = string(b.ch)
	b.position = b.readPosition
	b.readPosition += 1
}

func (b *BencodeReader) readChar() {
	// TODO(burmudar): this breaks when reading largers torrents
	// if b.ch == NULL_TERMINATOR && b.position >= 0 {
	// 	return
	// }
	if b.readPosition >= len(b.input) {
		b.ch = 0
	} else {
		b.ch = b.input[b.readPosition]
		b.curr = string(b.ch)
		b.position = b.readPosition
		b.readPosition += 1
	}
}

func (b *BencodeReader) peek() byte {
	if b.readPosition >= len(b.input) {
		return 0
	} else {
		return b.input[b.readPosition]
	}
}

// readN advances the curser n amount and returns the bytes read as well as how many bytes were read.
// If we reached end of input (EOI) before the requested amount of bytes were read, we return only what we read and the
// the amount of bytes read
func (b *BencodeReader) readN(n int) ([]byte, int) {
	data := []byte{}
	c := n
	for c != 0 {
		data = append(data, b.ch)
		b.readChar()
		c--
	}

	return data, n - c
}
