package types_test

import (
	"testing"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

func TestBlockSpec(t *testing.T) {
	torrent := types.Torrent{
		Name:        "Test Torrent",
		PieceLength: 16 * 1024, // 16 KiB
		PieceHashes: []string{"1", "2", "3"},
		Length:      ((16 * 1024) * 3) - 1, // 3 pieces with the last piece being slightly smaller
	}
	blockSize := 16 * 1024
	b := torrent.GetPieceSpec(blockSize)

	t.Logf("Length: %d", torrent.Length)
	t.Logf("LastBlockSize: %d", b.LastBlockSize)
	t.Logf("BlockSize: %d", b.BlockSize)

	if b.BlockSize != blockSize {
		t.Errorf("incorrect block size - got %d expected %d", b.BlockSize, blockSize)
	}
	if b.TotalBlocks != 2 {
		t.Errorf("incorrect total blocks - got %d expected %d", b.TotalBlocks, torrent.Length/b.BlockSize)
	}
	if b.TotalPieces != 3 {
		t.Errorf("incorrect total piece count - got %d wanted %d", b.TotalPieces, torrent.Length/torrent.PieceLength)
	}
	if b.PieceLength != torrent.PieceLength {
		t.Errorf("incorrect piece length - got %d wanted %d", b.PieceLength, 16*1024)
	}
	if b.LastPieceLength != (16*1024)-1 {
		t.Errorf("incorrect last piece length - got %d wanted %d", b.LastPieceLength, 16*1024-1)
	}
	if b.LastBlockSize != (16*1024)-1 {
		t.Errorf("incorrect last block size - got %d wanted %d", b.LastBlockSize, 16*1024-1)
	}
}
