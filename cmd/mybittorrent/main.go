package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	//bencode "github.com/jackpal/bencode-go" // Available if you need it!
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/encoding"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/peer"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/tracker"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
)

const PeerID = "00112233445566778899"

func printMetaInfo(m *types.FileMeta) {
	fmt.Printf("Tracker URL: %s\n", m.Announce)
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
	for _, p := range m.Pieces {
		fmt.Printf("%x\n", p)
	}
}

func GetPeers(m *types.FileMeta) (*types.PeerSpec, error) {
	client := tracker.NewClient()
	req, err := tracker.NewPeerRequest(PeerID, 6881, m)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer request: %v", err)
	}
	resp, err := client.PeersRequest(req)
	if err != nil {
		return nil, fmt.Errorf("peers request failure: %v", err)
	}

	return &types.PeerSpec{
		Peers:    resp.Peers,
		Interval: resp.Interval,
	}, nil
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

			client, err := peer.NewClient(PeerID)
			if err != nil {
				FatalExit("failed to create peer client: %v", err)
			}

			handshake, err := client.DoHandshake(t, p)
			if err != nil {
				FatalExit("%q handshake failed: %v", p.String(), err)
			}

			fmt.Printf("Peer ID: %s\n", hex.EncodeToString([]byte(handshake.PeerID)))
			fmt.Printf("Hash: %s\n", hex.EncodeToString([]byte(handshake.Hash[:])))

		}
	default:
		{
			FatalExit("Unknown command: " + command)
		}
	}

}
