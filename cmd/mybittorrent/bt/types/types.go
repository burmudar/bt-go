package types

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type FileInfo struct {
	Length int
	Paths  []string
}

type Torrent struct {
	Announce     string
	AnnounceList []string
	Name         string
	PieceLength  int
	Pieces       []string
	Length       int
	Files        []*FileInfo
	Hash         [20]byte
	RawInfo      map[string]interface{}
}

type Peer struct {
	IP   net.IP
	Port int
}

func ParsePeer(v string) (*Peer, error) {
	parts := strings.Split(v, ":")

	if len(parts) < 1 {
		return nil, fmt.Errorf("malformed peer value - expected IP:PORT format, got %s", v)
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed peer value - cannot convert port value %q", parts[1])
	}

	return &Peer{
		IP:   net.ParseIP(parts[0]),
		Port: port,
	}, nil
}

func (p *Peer) String() string {
	return fmt.Sprintf("%s:%d", p.IP.String(), p.Port)
}

type PeerSpec struct {
	Peers    []*Peer
	Interval int
}

func (m *Torrent) InfoDict() map[string]interface{} {
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
