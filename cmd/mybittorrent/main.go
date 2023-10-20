package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	if unicode.IsDigit(rune(bencodedString[0])) {

		colonIdx := strings.IndexRune(bencodedString, ':')

		encodedLen := bencodedString[:colonIdx]

		length, err := strconv.Atoi(encodedLen)
		if err != nil {
			return "", err
		}

		strLen := len(bencodedString) - colonIdx - 1
		if length > strLen {
			return "", fmt.Errorf("invalid length encoded - got %d but string is %d", length, strLen)
		}

		return bencodedString[colonIdx+1 : colonIdx+1+length], nil
	} else {
		return "", fmt.Errorf("Only strings are supported at the moment")
	}
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		value := os.Args[2]

		if r, err := decodeBencode(value); err == nil {
			r, err := json.Marshal(r)
			if err != nil {
				fmt.Printf("marshalling faliure: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(r))

		} else {
			fmt.Printf("decoding faliure: %v\n", err)
			os.Exit(1)
		}

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}

}
