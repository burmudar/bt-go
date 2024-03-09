package types_test

import (
	"testing"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/encoding"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type Block struct {
	Idx    int
	Length int
}

func TorrentBlocksFor(m *types.Torrent, piece, blockSize int) []Block {
	var (
		blocks []Block
		length = m.LengthOf(piece)
	)

	for i := 0; i < length; i += blockSize {
		b := Block{i, blockSize}

		// if the next block will push us over the length it means we're at the last block
		// so subtract i, which is the last block from length to get the last block size
		if i+blockSize >= length {
			b.Length = length - i
		}

		blocks = append(blocks, b)
	}

	return blocks

}

func TestBlocks(t *testing.T) {
	for _, tName := range []string{"testdata/sample.torrent", "testdata/sample2-debian-iso.torrent"} {
		t.Run(tName, func(t *testing.T) {
			torrent, err := encoding.DecodeTorrent(tName)
			if err != nil {
				t.Fatalf("failed to read torrent %q: %v", tName, err)
			}

			lastPiece := len(torrent.PieceHashes) - 1

			blocks := TorrentBlocksFor(torrent, lastPiece, types.DefaultBlockSize)
			plan := torrent.BlockPlan(lastPiece, types.DefaultBlockSize)

			if len(blocks) != plan.NumBlocks {
				t.Fatalf("incorrect total blocks - wanted %d got %d", len(blocks), plan.NumBlocks)
			}

			lastBlock := blocks[len(blocks)-1]

			if lastBlock.Length != plan.LastBlockSize {
				t.Fatalf("incorrect last block size - wanted %d got %d", lastBlock.Length, plan.LastBlockSize)
			}

			if len(blocks)-1 != plan.LastBlockIndex {
				t.Fatalf("incorrect last block index - wanted %d got %d", len(blocks)-1, plan.LastBlockIndex)
			}

			lastPieceLength := torrent.LengthOf(lastPiece)
			if lastPieceLength != plan.PieceLength {
				t.Fatalf("incorrect last piece length - wanted %d got %d", lastPieceLength, plan.PieceLength)
			}
		})
	}

}
