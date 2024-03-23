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

type BlockPlan struct {
	PieceIndex     int
	Hash           []byte
	PieceLength    int
	NumBlocks      int
	BlockSize      int
	LastBlockIndex int
	LastBlockSize  int
}

type Torrent struct {
	Announce     string
	AnnounceList []string
	Name         string
	PieceLength  int
	PieceHashes  []string
	Length       int
	Files        []*FileInfo
	Hash         [20]byte
	RawInfo      map[string]interface{}
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
	Hash  [20]byte
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
			"pieces":       strings.Join(m.PieceHashes, ""),
		}
	} else {
		info = map[string]interface{}{
			"name":         m.Name,
			"piece length": m.PieceLength,
			"pieces":       strings.Join(m.PieceHashes, ""),
			"files":        m.Files,
		}
	}

	return info
}

func (m *Torrent) GetPieceCount() int {
	return len(m.PieceHashes)
}

func (m *Torrent) LengthOfPiece(p int) int {
	if p == len(m.PieceHashes)-1 {
		return m.Length % m.PieceLength
	}
	return m.PieceLength
}

func (m *Torrent) HashForPiece(p int) []byte {
	if p >= len(m.PieceHashes) {
		return nil
	}

	return []byte(m.PieceHashes[p])
}

// BlockPlan determines the various block sizes one has to use for the given piece index and block size.
// In particular, care is taken to calculate the last block size of a particular piece since it might a Piece
// might not uniformly divide into the given blockSize, hence last block in the last piece might need to be
// a smaller size than the given blockSize.
func (m *Torrent) BlockPlan(pIndex, blockSize int) *BlockPlan {
	isLastPiece := (len(m.PieceHashes)-1 == pIndex)
	pieceLength := m.PieceLength
	lastBlockSize := blockSize

	if isLastPiece {
		lastPieceLength := m.Length % m.PieceLength
		if lastPieceLength != 0 {
			pieceLength = lastPieceLength
			lastBlockSize = pieceLength % blockSize
		}
	}

	numBlocks := bt.Ceil(pieceLength, blockSize)
	return &BlockPlan{
		PieceIndex:     pIndex,
		Hash:           []byte(m.PieceHashes[pIndex]),
		PieceLength:    pieceLength,
		NumBlocks:      numBlocks,
		BlockSize:      blockSize,
		LastBlockIndex: numBlocks - 1,
		LastBlockSize:  lastBlockSize,
	}

}

func (m *Torrent) AllBlockPlans(blockSize int) []*BlockPlan {
	all := []*BlockPlan{}
	for i := 0; i < len(m.PieceHashes); i++ {
		all = append(all, m.BlockPlan(i, blockSize))
	}

	return all
}

func (p *BlockPlan) BlockSizeFor(blockIndex int) int {
	if blockIndex == p.LastBlockIndex {
		return p.LastBlockSize
	}

	return p.BlockSize
}
