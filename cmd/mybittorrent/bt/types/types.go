package types

import "strings"

type FileInfo struct {
	Length int
	Paths  []string
}

type FileMeta struct {
	Announce    string
	Name        string
	PieceLength int
	Pieces      []string
	Length      int
	Files       []*FileInfo
	Hash        []byte
	RawInfo     map[string]interface{}
}

func (m *FileMeta) InfoDict() map[string]interface{} {
	var info map[string]interface{}
	if len(m.Files) == 0 {
		info = map[string]interface{}{
			"name":         m.Name,
			"length":       m.Length,
			"piece length": m.PieceLength,
			"pieces":       strings.Join(m.Pieces, ""),
		}
	} else {
		info = map[string]interface{}{
			"name":         m.Name,
			"piece length": m.PieceLength,
			"pieces":       strings.Join(m.Pieces, ""),
			"files":        m.Files,
		}
	}

	return info
}
