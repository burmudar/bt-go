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
func decodeBencode(bencodedString string) (interface{}, int, error) {
	switch {
	case unicode.IsLetter(rune(bencodedString[0])) && bencodedString[0] == 'l':
		{
			var values []interface{}
			var cursor string = bencodedString
			start := 1
			for cursor[start] != 'e' && len(cursor[start:]) != 1 {
				value, n, err := decodeBencode(cursor[start:])
				if err != nil {
					return "", 0, err
				}
				values = append(values, value)
				start += n + 1
			}

			return values, len(bencodedString) - 2, nil
		}
	case unicode.IsDigit(rune(bencodedString[0])):
		{
			colonIdx := strings.IndexRune(bencodedString, ':')
			if colonIdx < 0 {
				return "", 0, fmt.Errorf("invalid string encoding: %v", bencodedString)
			}

			encodedLen := bencodedString[:colonIdx]

			length, err := strconv.Atoi(encodedLen)
			if err != nil {
				return "", 0, err
			}

			strLen := len(bencodedString) - colonIdx - 1
			if length > strLen {
				return "", 0, fmt.Errorf("invalid length encoded - got %d but string is %d", length, strLen)
			}

			return bencodedString[colonIdx+1 : colonIdx+1+length], 1 + length, nil
		}
	case unicode.IsLetter(rune(bencodedString[0])) && bencodedString[0] == 'i':
		{
			end := strings.IndexRune(bencodedString, 'e')
			if end < 0 {
				return "", 0, fmt.Errorf("invalid integer encoding: %v", bencodedString)
			}
			num, err := strconv.Atoi(bencodedString[1:end])
			if err != nil {
				return "", 0, fmt.Errorf("failed to decode integer: %v", bencodedString)
			}
			return num, end, nil
		}
	default:
		{
			return "", 0, fmt.Errorf("Only strings are supported at the moment")
		}
	}
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		value := os.Args[2]

		if r, _, err := decodeBencode(value); err == nil {
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
