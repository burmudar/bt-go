package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"unicode"
)

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
			return "", fmt.Errorf("unknown decode tag: %v", r.ch)
		}
	}
}
