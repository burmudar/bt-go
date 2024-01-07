package manager

import (
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/peer"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/tracker"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type TorrentManager struct {
	PeerID  string
	Torrent *types.Torrent
	Tracker *tracker.TrackerClient

	peerRefreshCh <-chan bool
}

func NewTorrentManager(peerID string, t *types.Torrent) *TorrentManager {
	client := tracker.NewClient()

	return &TorrentManager{
		PeerID:  peerID,
		Torrent: t,
		Tracker: client,
	}
}

func (tm *TorrentManager) NewPeerPool() (*peer.Pool, error) {
	peers, err := tm.Tracker.GetPeers(tm.PeerID, 6881, tm.Torrent)
	if err != nil {
		return nil, err
	}

	return peer.NewPool(peers), nil
}
