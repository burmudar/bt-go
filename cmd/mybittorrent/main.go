package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	//bencode "github.com/jackpal/bencode-go" // Available if you need it!
	"os"
)

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		{
			value := os.Args[2]

			if result, err := decodeBencode(NewBencodeReader(value)); err == nil {
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
			t, err := decodeTorrent(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to read torrent %q: %v", os.Args[2], err)
			}

			fmt.Printf("Tracker URL: %s\n", t.Announce)
			if len(t.Files) == 0 {
				fmt.Printf("Length: %d\n", t.Length)
			} else {
				for _, f := range t.Files {
					fmt.Printf("Length: %d Files: %s\n", f.Length, strings.Join(f.Paths, " "))
				}
			}

			hash, err := t.InfoHash()
			if err != nil {
				fmt.Fprintf(os.Stderr, "calculating info hash error: %v", err)
			}
			fmt.Printf("Info Hash: %s\n", hex.EncodeToString(hash[:]))
		}
	default:
		{
			fmt.Println("Unknown command: " + command)
			os.Exit(1)
		}
	}

}
