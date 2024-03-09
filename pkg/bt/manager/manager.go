package manager

import (
	"fmt"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/peer"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/tracker"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type TorrentManager struct {
	PeerID  string
	Tracker *tracker.TrackerClient

	peerRefreshCh <-chan bool
}

func NewTorrentManager(peerID string) *TorrentManager {
	client := tracker.NewClient()

	return &TorrentManager{
		PeerID:  peerID,
		Tracker: client,
	}
}

func (tm *TorrentManager) newPeerManager(t *types.Torrent) (*peer.PeerManager, error) {
	fmt.Println("getting peers ...")
	peers, err := tm.Tracker.GetPeers(tm.PeerID, 6881, t)
	if err != nil {
		return nil, err
	}

	fmt.Println("Peers ", len(peers.Peers))

	return peer.NewPeerManager(tm.PeerID, peers.Peers), nil
}

func (tm *TorrentManager) Download(torrent *types.Torrent, dst string) error {
	p, err := tm.newPeerManager(torrent)
	if err != nil {
		return err
	}

	fmt.Println("initializing peer manager")
	if err := p.Init(torrent.Hash); err != nil {
		return err
	}
	fmt.Println("peer pool initialized")

	pieces, err := p.Download(torrent.AllBlockPlans(types.DefaultBlockSize))
	if err != nil {
		return err
	}

	fmt.Println("%d pieces downloaded", len(pieces))

	return nil
}
