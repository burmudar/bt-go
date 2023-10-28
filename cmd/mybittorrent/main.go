package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	//bencode "github.com/jackpal/bencode-go" // Available if you need it!
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/encoding"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/tracker"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
)

func printMetaInfo(m *types.FileMeta) {
	fmt.Printf("Tracker URL: %s\n", m.Announce)
	if len(m.Files) == 0 {
		fmt.Printf("Length: %d\n", m.Length)
	} else {
		for _, f := range m.Files {
			fmt.Printf("Length: %d Files: %s\n", f.Length, strings.Join(f.Paths, " "))
		}
	}

	hash, err := bt.InfoHash(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calculating info hash error: %v", err)
	}
	fmt.Printf("Info Hash: %s\n", hex.EncodeToString(hash[:]))
	fmt.Printf("Piece Length: %d\n", m.PieceLength)
	fmt.Println("Piece Hashes:")
	for _, p := range m.Pieces {
		fmt.Printf("%x\n", p)
	}
}

func FatalExit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
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
					FatalExit(fmt.Sprintf("marshalling faliure: %v\n", err))
				}
				fmt.Println(string(r))
			} else {
				FatalExit(fmt.Sprintf("decoding faliure: %v\n", err))
			}
		}
	case "info":
		{
			filename := os.Args[2]
			t, err := encoding.DecodeTorrent(filename)
			if err != nil {
				FatalExit(fmt.Sprintf("failed to read torrent %q: %v", os.Args[2], err))
			}
			printMetaInfo(t)
		}

	case "peers":
		{
			filename := os.Args[2]
			t, err := encoding.DecodeTorrent(filename)
			if err != nil {
				FatalExit(fmt.Sprintf("failed to read torrent %q: %v", os.Args[2], err))
			}

			client := tracker.NewClient()
			req, err := tracker.NewPeerRequest("00112233445566778899", 6881, t)
			if err != nil {
				FatalExit(fmt.Sprintf("failed to create peer request: %v", err))
			}
			resp, err := client.PeersRequest(req)
			if err != nil {
				FatalExit(fmt.Sprintf("peers request failure: %v", err))
			}

			for _, p := range resp.Peers {
				fmt.Printf("%s:%d\n", p.IP.String(), p.Port)
			}
		}
	default:
		{
			FatalExit("Unknown command: " + command)
		}
	}

}
