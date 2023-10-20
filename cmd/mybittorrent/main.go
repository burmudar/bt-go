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

func decodeList(encoded string) (interface{}, int, error) {
	values := make([]interface{}, 0)
	var cursor string = encoded
	start := 1
	for cursor[start] != 'e' && len(cursor[start:]) != 1 {
		value, n, err := decodeBencode(cursor[start:])
		if err != nil {
			return "", 0, err
		}
		values = append(values, value)
		start += n + 1
	}

	return values, len(encoded) - 2, nil
}

func decodeString(encoded string) (interface{}, int, error) {
	colonIdx := strings.IndexRune(encoded, ':')
	if colonIdx < 0 {
		return "", 0, fmt.Errorf("invalid string encoding: %v", encoded)
	}

	encodedLen := encoded[:colonIdx]

	length, err := strconv.Atoi(encodedLen)
	if err != nil {
		return "", 0, err
	}

	strLen := len(encoded) - colonIdx - 1
	if length > strLen {
		return "", 0, fmt.Errorf("invalid length encoded - got %d but string is %d", length, strLen)
	}

	return encoded[colonIdx+1 : colonIdx+1+length], 1 + length, nil
}

func decodeInt(encoded string) (interface{}, int, error) {
	end := strings.IndexRune(encoded, 'e')
	if end < 0 {
		return "", 0, fmt.Errorf("invalid integer encoding: %v", encoded)
	}
	num, err := strconv.Atoi(encoded[1:end])
	if err != nil {
		return "", 0, fmt.Errorf("failed to decode integer: %v", encoded)
	}
	return num, end, nil
}

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, int, error) {
	switch {
	case unicode.IsLetter(rune(bencodedString[0])) && bencodedString[0] == 'l':
		{
			return decodeList(bencodedString)
		}
	case unicode.IsDigit(rune(bencodedString[0])):
		{
			return decodeString(bencodedString)
		}
	case unicode.IsLetter(rune(bencodedString[0])) && bencodedString[0] == 'i':
		{
			return decodeInt(bencodedString)
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
