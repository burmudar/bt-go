package btencoding

import (
	"fmt"
	"testing"
)

func TestDecodeList(t *testing.T) {
	tt := []struct {
		name   string
		value  string
		assert func(interface{}) (bool, error)
	}{
		{
			"empty list - le",
			"le",
			func(v interface{}) (bool, error) {
				ll, ok := v.([]interface{})
				if !ok {
					return false, fmt.Errorf("invalid type - wanted list got %T", ll)
				}
				return len(ll) == 0, nil
			},
		},
		{
			"le with 3 elements",
			"l5:item15:item2i3ee",
			func(v interface{}) (bool, error) {
				ll, ok := v.([]interface{})
				if !ok {
					return false, fmt.Errorf("invalid type - wanted list got %T", ll)
				}
				return len(ll) == 3, nil
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			r := NewBencodeReader(tc.value)
			value, err := DecodeList(r)
			if err != nil {
				t.Fatalf("unexpected error decoding list: %v", err)
			}

			if ok, err := tc.assert(value); err != nil {
				t.Fatalf("assertion error: %v", err)
			} else if !ok {
				t.Fatalf("assertion failed - invalid type: %v", value)
			}
		})
	}
}

func TestDecodeDict(t *testing.T) {}

func TestDecodeBencode(t *testing.T) {}
