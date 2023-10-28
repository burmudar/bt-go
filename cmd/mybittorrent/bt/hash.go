package bt

import (
	"crypto/sha1"

	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/encoding"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
)

func InfoHash(m *types.FileMeta) ([20]byte, error) {
	w := encoding.NewBenEncoder()
	data, err := w.Encode(m.InfoDict())
	if err != nil {
		return [20]byte{}, err
	}
	return sha1.Sum(data), nil
}
