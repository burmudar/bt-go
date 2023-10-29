package encoding

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode"

	bt "github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
	//"github.com/jackpal/bencode-go"
)

func newFileInfo(value map[string]interface{}) *bt.FileInfo {
	var f bt.FileInfo

	f.Length = value["length"].(int)
	paths := []string{}
	for _, v := range value["path"].([]interface{}) {
		paths = append(paths, v.(string))
	}
	f.Paths = paths

	return &f
}

func DecodeTorrent(filename string) (*bt.FileMeta, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	reader := NewBencodeReader(string(raw))

	data, err := DecodeDict(reader)
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

	var m bt.FileMeta
	m.RawInfo = info
	m.Announce = dict["announce"].(string)

	if list, ok := dict["announce-list"].([]interface{}); ok {
		m.AnnounceList = make([]string, 0)
		for _, lvalue := range list {
			inner := lvalue.([]interface{})
			for _, v := range inner {
				m.AnnounceList = append(m.AnnounceList, v.(string))
			}
		}
	}

	if v, ok := info["name"]; ok {
		m.Name = v.(string)
	}

	m.PieceLength = info["piece length"].(int)
	// Parse the pieces
	piecesStr := info["pieces"].(string)
	buf := bytes.NewBufferString(piecesStr)

	m.Pieces = []string{}
	for {
		data := make([]byte, 20)
		n, err := buf.Read(data)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if n > 0 {
			m.Pieces = append(m.Pieces, string(data[:n]))
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}

	if v, ok := info["length"]; ok {
		m.Length = v.(int)
	} else {
		m.Files = make([]*bt.FileInfo, 0)
		fileList := info["files"].([]interface{})

		for _, item := range fileList {
			fileDict := item.(map[string]interface{})
			m.Files = append(m.Files, newFileInfo(fileDict))
		}
	}

	m.Hash, err = hash(m.InfoDict())
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func hash(v map[string]interface{}) ([20]byte, error) {
	w := NewBenEncoder()
	h, err := w.Encode(v)
	if err != nil {
		return [20]byte{}, err
	}
	return sha1.Sum(h), nil
}

func DecodeDict(r *BencodeReader) (interface{}, error) {
	dict := make(map[string]interface{}, 0)
	r.ReadChar() // move past 'd'
	for r.ch != 'e' && r.Err == nil {
		k, err := DecodeBencode(r)
		if err != nil {
			return "", err
		}
		var key string
		if k, ok := k.(string); !ok {
			return "", fmt.Errorf("expected string key but got %q - %x", k, r.ch)
		} else {
			key = k
		}
		v, err := DecodeBencode(r)
		if err != nil {
			return "", err
		}
		dict[key] = v
	}

	r.ReadChar() // advance past 'e'
	return dict, nil
}

func DecodeList(r *BencodeReader) (interface{}, error) {
	values := make([]interface{}, 0)
	// advance past 'l'
	if r.ch != 'l' {
		return nil, fmt.Errorf("current ch '%v' - expected 'l'", string(r.ch))
	}
	r.ReadChar()
	for r.ch != 'e' && r.Err == nil {
		v, err := DecodeBencode(r)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	r.ReadChar() // move past 'e'

	return values, nil
}

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func DecodeBencode(r *BencodeReader) (interface{}, error) {
	switch {
	case r.ch == 'd':
		{
			return DecodeDict(r)
		}
	case r.ch == 'l':
		{
			return DecodeList(r)
		}
	case r.ch == 'i':
		{
			return r.ReadInt()
		}
	case unicode.IsDigit(rune(r.ch)):
		{
			return r.ReadString()
		}
	default:
		{
			return "", fmt.Errorf("unknown decode tag: %v", string(r.ch))
		}
	}
}
