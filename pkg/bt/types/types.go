package types

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt"
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

type PieceSpec struct {
	TotalBlocks     int
	TotalPieces     int
	BlockSize       int
	LastBlockSize   int
	LastPieceIndex  int
	LastPieceLength int
}

type Peer struct {
	IP   net.IP
	Port int
}
type Piece struct {
	Index int
	Peer  Peer
	Size  int
	Data  []byte
}

func ParsePeer(v string) (*Peer, error) {
	parts := strings.Split(v, ":")
	println(v)

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

func (m *Torrent) GetPieceSpec(blockSize int) *PieceSpec {
	var b PieceSpec
	b.BlockSize = bt.Min(m.PieceLength, blockSize)
	b.TotalPieces = len(m.Pieces)
	b.TotalBlocks = m.Length / b.BlockSize
	b.LastPieceIndex = b.TotalPieces - 1

	lastPieceLength := m.Length % m.PieceLength
	println(m.Length, " % ", m.PieceLength, " = ", lastPieceLength)
	// last Piece is not the same size as other Pieces, so we have to handle it differently
	if lastPieceLength != 0 {
		b.LastPieceLength = lastPieceLength
		b.LastBlockSize = b.LastPieceLength % b.BlockSize
	} else {
		b.LastPieceLength = m.PieceLength
		b.LastBlockSize = b.BlockSize
	}
	return &b
}
