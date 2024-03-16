package manager

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
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

type PieceDownloadFailedErr struct {
	BlockPlan *types.BlockPlan
}

func (p *PieceDownloadFailedErr) Error() string {
	return p.String()
}

func (p *PieceDownloadFailedErr) String() string {
	return fmt.Sprintf("piece %d failed to download", p.BlockPlan.PieceIndex)
}

type DownloaderPool struct {
	Size int

	clientPool peer.Pool

	errC     chan error
	workC    chan *types.BlockPlan
	complete chan *types.Piece

	wg  *sync.WaitGroup
	sem *semaphore.Weighted

	Result []*types.Piece
}

func NewDownloaderPool(s int, clientPool peer.Pool) *DownloaderPool {
	return &DownloaderPool{
		Size:       s,
		clientPool: clientPool,
		errC:       make(chan error),
		workC:      make(chan *types.BlockPlan),
		complete:   make(chan *types.Piece),
		wg:         &sync.WaitGroup{},
		sem:        semaphore.NewWeighted(int64(s)),
	}
}

func (dp *DownloaderPool) Start() {
	for i := 0; i < dp.Size; i++ {
		dp.wg.Add(1)
		go dp.startWorker(i)
	}
}

func (dp *DownloaderPool) Wait() ([]*types.Piece, error) {
	var allPieces = []*types.Piece{}
	var allErrs error
loop:
	for {
		select {
		case p := <-dp.complete:
			allPieces = append(allPieces, p)
			break loop
		case err := <-dp.errC:
			if pErr, ok := err.(*PieceDownloadFailedErr); ok {
				dp.workC <- pErr.BlockPlan
			}
			allErrs = multierror.Append(allErrs, err)
		}
	}

	close(dp.workC)
	dp.wg.Wait()

	dp.Close()

	return allPieces, allErrs

}

func (dp *DownloaderPool) Close() {
	close(dp.workC)
	close(dp.errC)
	close(dp.complete)
}

func (dp *DownloaderPool) Download(work *types.BlockPlan) {
	dp.workC <- work
}

func (dp *DownloaderPool) startWorker(id int) {
	defer dp.wg.Done()
	for piecePlan := range dp.workC {
		ctx := context.Background()
		if err := dp.sem.Acquire(ctx, 1); err != nil {
			dp.errC <- fmt.Errorf("[downloader %d] failed to acquire semaphore for download: %w", id, err)
			continue
		}

		innerCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		client, release, err := dp.clientPool.Get(innerCtx)
		if err != nil {
			dp.errC <- fmt.Errorf("[downloader %d] failed to retrieve client from pool: %w", id, err)
			dp.sem.Release(1)
			cancel()
			release()
			continue
		}

		fmt.Printf("[downloader %d] downloading piece %d\n", id, piecePlan.PieceIndex)
		piece, err := client.DownloadPiece(piecePlan)
		if err != nil {
			dp.errC <- &PieceDownloadFailedErr{BlockPlan: piecePlan}
			dp.sem.Release(1)
			cancel()
			release()
			continue
		}

		dp.complete <- piece
		dp.sem.Release(1)
		cancel()
		release()
	}
}

func download(p peer.Pool, torrent *types.Torrent) ([]*types.Piece, error) {
	plans := torrent.AllBlockPlans(MaxBlockSize)

	var dp = NewDownloaderPool(5, p)

	dp.Start()

	for _, plan := range plans {
		dp.Download(plan)
	}

	pieces, err := dp.Wait()

	fmt.Printf("%d pieces downloaded\n", len(pieces))

	sort.SliceStable(pieces, func(a, b int) bool {
		p1 := pieces[a]
		p2 := pieces[b]

		return p1.Index <= p2.Index
	})

	return pieces, err
}
