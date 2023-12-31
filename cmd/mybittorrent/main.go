package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	//bencode "github.com/jackpal/bencode-go" // Available if you need it!
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/encoding"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/peer"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/tracker"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

const PeerID = "00112233445566778899"

func printMetaInfo(m *types.Torrent) {
	fmt.Printf("Tracker URL: %s\n", m.Announce)
	if len(m.AnnounceList) > 0 {
		fmt.Printf("AnnounceList:\n%s\n", strings.Join(m.AnnounceList, "\n"))
	}
	if len(m.Files) == 0 {
		fmt.Printf("Length: %d\n", m.Length)
	} else {
		for _, f := range m.Files {
			fmt.Printf("Length: %d Files: %s\n", f.Length, strings.Join(f.Paths, " "))
		}
	}

	fmt.Printf("Info Hash: %s\n", hex.EncodeToString(m.Hash[:]))
	fmt.Printf("Piece Length: %d\n", m.PieceLength)
	fmt.Println("Piece Hashes:")
	for _, p := range m.PieceHashes {
		fmt.Printf("%x\n", p)
	}
}

func GetPeers(m *types.Torrent) (*types.PeerSpec, error) {
	client := tracker.NewClient()
	return client.GetPeers(PeerID, 6881, m)
}

func FatalExit(format string, args ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		{
			value := os.Args[2]

			r := encoding.NewBencodeReader(value)
			if result, err := encoding.DecodeBencode(r); err == nil {
				r, err := json.Marshal(result)
				if err != nil {
					FatalExit("marshalling faliure: %v\n", err)
				}
				fmt.Println(string(r))
			} else {
				FatalExit("decoding faliure: %v\n", err)
			}
		}
	case "info":
		{
			filename := os.Args[2]
			t, err := encoding.DecodeTorrent(filename)
			if err != nil {
				FatalExit("failed to read torrent %q: %v", os.Args[2], err)
			}
			printMetaInfo(t)
		}

	case "peers":
		{
			filename := os.Args[2]
			t, err := encoding.DecodeTorrent(filename)
			if err != nil {
				FatalExit("failed to read torrent %q: %v", os.Args[2], err)
			}

			spec, err := GetPeers(t)
			if err != nil {
				FatalExit("failed to get peers: %v", err)
			}

			for _, p := range spec.Peers {
				fmt.Printf("%s:%d\n", p.IP.String(), p.Port)
			}
		}
	case "handshake":
		{
			filename := os.Args[2]
			p, err := types.ParsePeer(os.Args[3])
			if err != nil {
				FatalExit("invalid peer: %v", err)
			}
			t, err := encoding.DecodeTorrent(filename)
			if err != nil {
				FatalExit("failed to read torrent %q: %v", os.Args[2], err)
			}

			client := peer.NewClient(PeerID)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err = client.Connect(ctx, p); err != nil {
				FatalExit("failed to connect to client: %v", err)
			}
			defer client.Close()

			handshake, err := client.Handshake(t)
			if err != nil {
				FatalExit("%q handshake failed: %v", p.String(), err)
			}

			fmt.Printf("Peer ID: %s\n", hex.EncodeToString([]byte(handshake.PeerID)))
			fmt.Printf("Hash: %s\n", hex.EncodeToString([]byte(handshake.Hash[:])))

		}
	case "download_piece":
		{
			dst := os.Args[3]
			torrentFile := os.Args[4]
			pieceIdx, err := strconv.Atoi(os.Args[5])
			if err != nil {
				FatalExit("failed to convert piece index to integer: %v", err)
			}
			t, err := encoding.DecodeTorrent(torrentFile)
			if err != nil {
				FatalExit("failed to read torrent %q: %v", os.Args[2], err)
			}

			peers, err := GetPeers(t)
			if err != nil {
				FatalExit("failed to get peers: %v", os.Args[2], err)
			}

			client, err := peer.NewHandshakedClient(PeerID, peers.Peers[0], t)
			if err != nil {
				FatalExit("failed to create handshaked peer client: %v", err)
			}

			if _, err := client.BitField(); err != nil {
				FatalExit("failed to connect to client: %v", err)
			}

			fmt.Printf("[File %d] Downloading Piece %d from peer %s [%x] (%d)\n", t.Length, pieceIdx, client.Peer.String(), t.PieceHashes[pieceIdx], t.PieceLength)
			if b, err := client.DownloadPiece(t, pieceIdx); err != nil {
				FatalExit("piece download failure: %v", err)
			} else {
				fmt.Printf("Piece %d downloaded successfully from peer %s [%x] (%d)\n", pieceIdx, client.Peer.String(), sha1.Sum(b.Data), len(b.Data))
				fd, err := os.Create(dst)
				if err != nil {
					FatalExit("failed to create destination file: %v", err)
				}
				defer fd.Close()
				io.Copy(fd, bytes.NewReader(b.Data))
				fmt.Printf("Piece %d downloaded to %s\n", b.Index, dst)
			}
		}
	default:
		{
			FatalExit("Unknown command: " + command)
		}
	}

}
