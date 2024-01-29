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

func (tm *TorrentManager) newPeerPool(t *types.Torrent) (*peer.Pool, error) {
	fmt.Println("getting peers ...")
	peers, err := tm.Tracker.GetPeers(tm.PeerID, 6881, t)
	if err != nil {
		return nil, err
	}

	fmt.Println("Peers ", len(peers.Peers))

	return peer.NewPool(tm.PeerID, peers), nil
}

func (tm *TorrentManager) Download(torrent *types.Torrent, dst string) error {
	p, err := tm.newPeerPool(torrent)
	if err != nil {
		return err
	}

	fmt.Println("initializing peer pool")
	if canProcess, err := p.Init(torrent); !canProcess && err != nil {
		return err
	} else if err != nil {
		fmt.Printf("some errors occured during pool initialization: %v\n", err)
	}
	fmt.Println("peer pool initialized")

	fmt.Println("starting download")
	<-p.Download(torrent, 16*1024, dst)
	fmt.Println("download complete")
	return nil
}
