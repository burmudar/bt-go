package manager

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/peer"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/tracker"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
	"golang.org/x/sync/semaphore"
)

const MaxBlockSize = 16 * 1024

type TorrentManager struct {
	PeerID  string
	Tracker *tracker.TrackerClient
}

func NewTorrentManager(peerID string, torrent *types.Torrent) *TorrentManager {
	client := tracker.NewClient()

	return &TorrentManager{
		PeerID:  peerID,
		Tracker: client,
	}
}

func (tm *TorrentManager) newPeerPool(t *types.Torrent) (peer.Pool, error) {
	fmt.Println("getting peers ...")
	peers, err := tm.Tracker.GetPeers(tm.PeerID, 6881, t)
	if err != nil {
		return nil, err
	}

	fmt.Println("Peers ", len(peers.Peers))

	return peer.NewPool(tm.PeerID, peers, t)
}

func (tm *TorrentManager) Download(torrent *types.Torrent, dst string) error {
	p, err := tm.newPeerPool(torrent)
	if err != nil {
		return err
	}

	fmt.Println("starting download")
	all, err := download(p, torrent)
	if err != nil {
		fmt.Println("download failed")
		return err
	}
	fmt.Println("download complete")

	fd, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fd.Close()
	for _, piece := range all {
		_, err := fd.Write(piece.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

func download(p peer.Pool, torrent *types.Torrent) ([]*types.Piece, error) {
	plans := torrent.AllBlockPlans(MaxBlockSize)
	queue := types.NewSyncQueue[*types.BlockPlan]()
	queue.AddAll(plans...)

	allPieces := []*types.Piece{}

	var downloaded = make(chan *types.Piece, len(plans))
	var done = make(chan struct{})
	var errC = make(chan error)
	var allErrs error

	grp := sync.WaitGroup{}
	go func() {
	loop:
		for {
			select {
			case p := <-downloaded:
				allPieces = append(allPieces, p)
			case err := <-errC:
				allErrs = multierror.Append(allErrs, err)
			case <-done:
				break loop
			}
		}
	}()

	sem := semaphore.NewWeighted(5)
	var count atomic.Int32
	count.Swap(int32(len(plans)))
	fmt.Println("piece queue is: ", queue.Size())
	for !queue.IsEmpty() {
		piecePlan, ok := queue.Pop()
		if !ok {
			fmt.Println("failed to retrieve piece from queue")
			break
		}

		grp.Add(1)
		go func(piecePlan *types.BlockPlan) {
			defer grp.Done()

			ctx := context.Background()
			if err := sem.Acquire(ctx, 1); err != nil {
				errC <- fmt.Errorf("[piece %d] failed to acquire semaphore for download: %w", piecePlan.PieceIndex, err)
				return
			}
			defer sem.Release(1)

			innerCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			client, release, err := p.Get(innerCtx)
			defer release()
			if err != nil {
				errC <- fmt.Errorf("[piece %d] failed to retrieve client from pool: %w", piecePlan.PieceIndex, err)
				return
			}

			fmt.Printf("downloading piece %d\n", piecePlan.PieceIndex)
			piece, err := client.DownloadPiece(piecePlan)
			if err != nil {
				fmt.Printf("\n\nfailed to download piece: %v\n\n\n", err)
				queue.Add(piecePlan)
				return
			}
			count.Add(-1)

			fmt.Printf("\n\n### %d Left\n", count.Load())

			downloaded <- piece
			fmt.Println("<--- go routine end --->")
		}(piecePlan)

	}

	grp.Wait()
	close(done)
	fmt.Printf("%d pieces downloaded\n", len(allPieces))

	sort.SliceStable(allPieces, func(a, b int) bool {
		p1 := allPieces[a]
		p2 := allPieces[b]

		return p1.Index <= p2.Index
	})

	return allPieces, allErrs
}
