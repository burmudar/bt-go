package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	//_bencode "github.com/jackpal/bencode-go" // Available if you need it!
	"os"
)

// Types
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

type BenEncoder struct {
	buf *bytes.Buffer
}

type FileInfo struct {
	Length int
	Paths  []string
}

type Meta struct {
	Announce    string
	Name        string
	PieceLength int
	Pieces      []string
	Length      int
	Files       []*FileInfo
	Hash        []byte
	RawInfo     map[string]interface{}
}

// Decoder
func newFileInfo(value map[string]interface{}) *FileInfo {
	var f FileInfo

	f.Length = value["length"].(int)
	paths := []string{}
	for _, v := range value["path"].([]interface{}) {
		paths = append(paths, v.(string))
	}
	f.Paths = paths

	return &f
}

func decodeTorrent(filename string) (*Meta, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	reader := NewBencodeReader(string(raw))

	data, err := decodeDict(reader)
	if err != nil {
		return nil, err
	}

	dict := data.(map[string]interface{})
	var info map[string]interface{}
	if v, ok := dict["info"].(map[string]interface{}); ok {
		info = v
	} else {
		return nil, fmt.Errorf("info dict not found")
	}

	var m Meta
	m.RawInfo = info
	m.Announce = dict["announce"].(string)
	if v, ok := info["name"]; ok {
		m.Name = v.(string)
	}

	m.PieceLength = info["piece length"].(int)
	// Parse the pieces
	piecesStr := info["pieces"].(string)
	buf := bytes.NewBufferString(piecesStr)

	chunks := len(piecesStr) / 20
	m.Pieces = make([]string, 0, chunks)
	for i := 0; i < chunks; i++ {
		data := make([]byte, 20)
		_, err := buf.Read(data)
		if err != nil {
			return nil, err
		}

		m.Pieces = append(m.Pieces, string(data))
	}

	if len(m.Pieces) != chunks {
		panic("incorrect pieces length")
	}

	if v, ok := info["length"]; ok {
		m.Length = v.(int)
	} else {
		m.Files = make([]*FileInfo, 0)
		fileList := info["files"].([]interface{})

		for _, item := range fileList {
			fileDict := item.(map[string]interface{})
			m.Files = append(m.Files, newFileInfo(fileDict))
		}
	}

	return &m, nil
}

func decodeDict(r *BencodeReader) (interface{}, error) {
	dict := make(map[string]interface{}, 0)
	r.readChar() // move past 'd'
	for r.ch != 'e' && r.ch != EOI {
		k, err := decodeBencode(r)
		if err != nil {
			return "", err
		}
		var key string
		if k, ok := k.(string); !ok {
			return "", fmt.Errorf("expected string key but got %q at %d - %x", k, r.position, r.ch)
		} else {
			key = k
		}
		v, err := decodeBencode(r)
		if err != nil {
			return "", err
		}
		dict[key] = v
	}

	r.readChar() // advance past 'e'
	return dict, nil
}

func decodeList(r *BencodeReader) (interface{}, error) {
	values := make([]interface{}, 0)
	r.readChar() // advance past 'l'
	for r.ch != 'e' && r.ch != EOI {
		v, err := decodeBencode(r)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	r.readChar() // move past 'e'

	return values, nil
}

func decodeString(r *BencodeReader) (interface{}, error) {
	num := []byte{}
	for r.ch != ':' {
		num = append(num, byte(r.ch))
		r.readChar()
	}
	r.readChar()
	length, err := strconv.Atoi(string(num))
	if err != nil {
		return nil, err
	}
	// now we can read the string with the length that we got
	data, _ := r.readN(length)
	return string(data), nil
}

func decodeInt(r *BencodeReader) (interface{}, error) {
	r.readChar() // move past 'i'
	num := []byte{}
	for r.ch != 'e' {
		num = append(num, byte(r.ch))
		r.readChar()
	}
	r.readChar() // move past 'e'

	return strconv.Atoi(string(num))
}

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(r *BencodeReader) (interface{}, error) {
	switch {
	case r.ch == 'd':
		{
			return decodeDict(r)
		}
	case r.ch == 'l':
		{
			return decodeList(r)
		}
	case r.ch == 'i':
		{
			return decodeInt(r)
		}
	case unicode.IsDigit(rune(r.ch)):
		{
			return decodeString(r)
		}
	default:
		{
			return "", fmt.Errorf("only strings are supported at the moment: %x", r.ch)
		}
	}
}

// Encoder
func NewBenEncoder() *BenEncoder {
	return &BenEncoder{
		buf: bytes.NewBuffer(nil),
	}
}

func (b *BenEncoder) encode(value interface{}) ([]byte, error) {

	switch v := value.(type) {
	case string:
		b.encodeString(v)
	case int:
		b.encodeInt(v)
	case []interface{}:
		b.encodeList(v)
	case map[string]interface{}:
		b.encodeDict(v)
	}

	return b.buf.Bytes(), nil
}

func (b *BenEncoder) encodeInt(v int) {
	fmt.Fprintf(b.buf, "i%de", v)
}

func (b *BenEncoder) encodeString(v string) {
	fmt.Fprintf(b.buf, "%d:%s", len(v), v)
}

func (b *BenEncoder) encodeList(list []interface{}) {
	fmt.Fprintf(b.buf, "l")
	for _, i := range list {
		b.encode(i)
	}
	fmt.Fprintf(b.buf, "e")
}

func (b *BenEncoder) encodeDict(dict map[string]interface{}) {
	fmt.Fprintf(b.buf, "l")
	// bencoding requries keys to be lexographically sorted
	keys := []string{}
	for k := range dict {
		keys = append(keys, k)
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i] <= keys[j]
	})
	for _, k := range keys {
		b.encode(k)
		b.encode(dict[k])
	}
	fmt.Fprintf(b.buf, "e")
}

// Reader
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
	if b.ch == NULL_TERMINATOR && b.position >= 0 {
		return
	}
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
	c := n - 1
	for c >= 0 && b.ch != EOI {
		data = append(data, b.ch)
		b.readChar()
		c--
	}

	return data, n - c
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		{
			value := os.Args[2]

			if result, err := decodeBencode(NewBencodeReader(value)); err == nil {
				r, err := json.Marshal(result)
				if err != nil {
					fmt.Printf("marshalling faliure: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(string(r))
				w := NewBenEncoder()
				data, _ := w.encode(result)
				fmt.Printf("encoded: %s\n", string(data))

			} else {
				fmt.Printf("decoding faliure: %v\n", err)
				os.Exit(1)
			}
		}
	case "info":
		{
			t, err := decodeTorrent(os.Args[2])
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to read torrent %q: %v", os.Args[2], err)
			}

			fmt.Printf("Tracker URL: %s\n", t.Announce)
			if len(t.Files) == 0 {
				fmt.Printf("Length: %d\n", t.Length)
			} else {
				for _, f := range t.Files {
					fmt.Printf("Length: %d Files: %s\n", f.Length, strings.Join(f.Paths, " "))
				}
			}
			w := NewBenEncoder()
			d, _ := w.encode(t.RawInfo)
			fmt.Printf("Info Hash: %x\n", sha1.Sum(d))
		}
	default:
		{
			fmt.Println("Unknown command: " + command)
			os.Exit(1)
		}
	}

}
