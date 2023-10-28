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
					fmt.Printf("marshalling faliure: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(string(r))
			} else {
				fmt.Printf("decoding faliure: %v\n", err)
				os.Exit(1)
			}
		}
	case "info":
		{
			filename := os.Args[2]
			t, err := encoding.DecodeTorrent(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to read torrent %q: %v", os.Args[2], err)
			}
			printMetaInfo(t)
		}
	default:
		{
			fmt.Println("Unknown command: " + command)
			os.Exit(1)
		}
	}

}
