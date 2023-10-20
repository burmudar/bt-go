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

func decodeDict(encoded string) (interface{}, int, error) {
	dict := make(map[string]interface{}, 0)
	var cursor string = encoded
	idx := 1
	for cursor[idx] != 'e' && len(cursor[idx:]) != 1 {
		k, n, err := decodeString(cursor[idx:])
		if err != nil {
			return "", 0, err
		}
		idx += n + 1
		var key string
		if k, ok := k.(string); !ok {
			return "", idx, fmt.Errorf("expected string key but got %q", k)
		} else {
			key = k
		}

		v, n, err := decodeBencode(cursor[idx:])
		if err != nil {
			return "", 0, err
		}
		dict[key] = v
		idx += n + 1

	}

	return dict, idx, nil
}

func decodeList(encoded string) (interface{}, int, error) {
	values := make([]interface{}, 0)
	var cursor string = encoded
	idx := 1
	for cursor[idx] != 'e' && len(cursor[idx:]) != 1 {
		value, n, err := decodeBencode(cursor[idx:])
		if err != nil {
			return "", 0, err
		}
		values = append(values, value)
		idx += n + 1
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
	case unicode.IsLetter(rune(bencodedString[0])) && bencodedString[0] == 'd':
		{
			return decodeDict(bencodedString)
		}
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
