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

type PeerClientErr struct {
	Err       error
	BlockPlan *types.BlockPlan
}

func (p *PeerClientErr) Error() string {
	return p.String()
}

func (p *PeerClientErr) String() string {
	return fmt.Sprintf("peer client error: %v", p.Err)
}

type PieceDownloadFailedErr struct {
	Err       error
	BlockPlan *types.BlockPlan
}

func (p *PieceDownloadFailedErr) Error() string {
	return p.String()
}

func (p *PieceDownloadFailedErr) String() string {
	return fmt.Sprintf("piece %d failed to download: %v", p.BlockPlan.PieceIndex, p.Err)
}

type DownloaderPool struct {
	Size int

	clientPool peer.Pool

	errC     chan error
	workC    chan *types.BlockPlan
	complete chan *types.Piece

	count atomic.Int64
	wg    *sync.WaitGroup
	sem   *semaphore.Weighted

	Result []*types.Piece
}

func NewDownloaderPool(s int, clientPool peer.Pool) *DownloaderPool {
	return &DownloaderPool{
		Size:       s,
		clientPool: clientPool,
		errC:       make(chan error, 5),
		workC:      make(chan *types.BlockPlan),
		complete:   make(chan *types.Piece, s),
		wg:         &sync.WaitGroup{},
		sem:        semaphore.NewWeighted(int64(s)),
	}
}

func (dp *DownloaderPool) Start() chan peer.Result[[]*types.Piece] {
	for i := 0; i < dp.Size; i++ {
		dp.wg.Add(1)
		go dp.startWorker(i)
	}

	return dp.resultCh()
}

func (dp *DownloaderPool) resultCh() chan peer.Result[[]*types.Piece] {

	resultC := make(chan peer.Result[[]*types.Piece])
	go func() {
		var allPieces = []*types.Piece{}
		var allErrs error
	loop:
		for {
			select {
			case p := <-dp.complete:
				allPieces = append(allPieces, p)
				total := dp.count.Load()
				if total == int64(len(allPieces)) {
					break loop
				} else {
					fmt.Printf("--- (%d/%d)\n", len(allPieces), total)
				}
			case err := <-dp.errC:
				switch e := err.(type) {
				case *PieceDownloadFailedErr:
					fmt.Printf("--- Piece %d failed - Retrying ---\n", e.BlockPlan.PieceLength)
					dp.count.Add(-1)
					go dp.Download(e.BlockPlan)
				case *PeerClientErr:
					fmt.Printf("--- Peer Client err - Retrying piece %d ---\n", e.BlockPlan.PieceLength)
					dp.count.Add(-1)
					go dp.Download(e.BlockPlan)
				default:
					allErrs = multierror.Append(allErrs, err)

				}
			}
		}
		close(dp.workC)
		dp.wg.Wait()

		close(dp.errC)
		close(dp.complete)

		resultC <- peer.Result[[]*types.Piece]{
			R:   allPieces,
			Err: allErrs,
		}
	}()

	return resultC

}

func (dp *DownloaderPool) Download(work *types.BlockPlan) {
	dp.count.Add(1)
	dp.workC <- work
}

func (dp *DownloaderPool) doWorkerDownload(id int, piecePlan *types.BlockPlan) error {
	defer fmt.Printf("[downloader %d] work complete\n", id)
	fmt.Printf("[downloader %d] %d piece\n", id, piecePlan.PieceIndex)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	fmt.Printf("[downloader %d] acuiring semaphore\n", id)
	if err := dp.sem.Acquire(ctx, 1); err != nil {
		cancel()
		return fmt.Errorf("[downloader %d] failed to acquire semaphore for download: %w", id, err)
	}
	defer dp.sem.Release(1)
	defer cancel()

	innerCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	fmt.Printf("[downloader %d] acuiring client\n", id)
	client, release, err := dp.clientPool.Get(innerCtx)
	defer release()
	if err != nil {
		fmt.Printf("[downloader %d] failed to get client (PeerClientErr): %v\n", id, err)
		return &PeerClientErr{
			Err:       fmt.Errorf("[downloader %d] failed to retrieve client from pool: %w", id, err),
			BlockPlan: piecePlan,
		}
	}

	fmt.Printf("[downloader %d] downloading piece %d\n", id, piecePlan.PieceIndex)
	piece, err := client.DownloadPiece(piecePlan)
	if err != nil {
		switch err {
		case peer.ErrChannelClosed:
			fmt.Printf("[downloader %d] client channel closed\n", id)
			return &PeerClientErr{
				Err:       err,
				BlockPlan: piecePlan,
			}
		case peer.ErrPieceUnavailable:
			fmt.Printf("[downloader %d] piece unavailable %d\n", id, piecePlan.PieceIndex)
			fallthrough
		default:
			return &PieceDownloadFailedErr{BlockPlan: piecePlan, Err: err}
		}
	}
	dp.complete <- piece
	return nil
}

func (dp *DownloaderPool) startWorker(id int) {
	fmt.Printf("<<<<<<<<<<<<<<<< WORKER %d [START] >>>>>>>>>>>>>>>>>>>>\n", id)
	defer dp.wg.Done()
	for piecePlan := range dp.workC {
		fmt.Printf("<<<<<<<<<<<<<<<< WORKER %d [WORK Piece %d] >>>>>>>>>>>>>>>>>>>>\n", id, piecePlan.PieceIndex)
		if err := dp.doWorkerDownload(id, piecePlan); err != nil {

			dp.errC <- err
			fmt.Printf("<<<<<<<<<<<<<<<< WORKER %d [WORK ERR ] >>>>>>>>>>>>>>>>>>>>\n", id)
		}
	}

	fmt.Printf("<<<<<<<<<<<<<<<< WORKER %d [QUIT] >>>>>>>>>>>>>>>>>>>>\n", id)
}

func download(p peer.Pool, torrent *types.Torrent) ([]*types.Piece, error) {
	plans := torrent.AllBlockPlans(MaxBlockSize)

	var dp = NewDownloaderPool(10, p)

	results := dp.Start()

	for _, plan := range plans {
		dp.Download(plan)
	}

	downloadResult := <-results
	if downloadResult.Err != nil {
		fmt.Printf("<< ERR: %v >>", downloadResult.Err)
	}

	pieces := downloadResult.R

	fmt.Printf("%d pieces downloaded\n", len(pieces))

	sort.SliceStable(pieces, func(a, b int) bool {
		p1 := pieces[a]
		p2 := pieces[b]

		return p1.Index <= p2.Index
	})

	return pieces, downloadResult.Err
}
