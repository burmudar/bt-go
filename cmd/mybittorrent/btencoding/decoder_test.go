package btencoding

import (
	"reflect"
	"testing"
)

func TestDecodeList(t *testing.T) {
	tt := []struct {
		name     string
		value    string
		expected []interface{}
	}{
		{
			"empty list - le",
			"le",
			[]interface{}{},
		},
		{
			"le with 3 elements",
			"l5:item15:item2i3ee",
			[]interface{}{"item1", "item2", 3},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			r := NewBencodeReader(tc.value)
			value, err := DecodeList(r)
			if err != nil {
				t.Fatalf("unexpected error decoding list: %v", err)
			}

			list, ok := value.([]interface{})
			if !ok {
				t.Fatal("failed to cast value to list")
			}
			if len(list) != len(tc.expected) {
				t.Errorf("expected list of length %d got %d", len(list), len(tc.expected))
			}
			for i := range tc.expected {
				a := list[i]
				b := tc.expected[i]

				if reflect.TypeOf(a) != reflect.TypeOf(b) {
					t.Errorf("types differ: expected %T, got %T", a, b)
				} else if a != b {
					t.Errorf("incorrect values: expected %v got %v", a, b)
				}
			}
		})
	}
}

func TestDecodeNestedLists(t *testing.T) {
	tt := []struct {
		name         string
		value        string
		nesting      int
		lastExpected []int
	}{
		{
			"nesting 2 levels deep",
			"llee",
			1,
			[]int{},
		},
		{
			"nesting 3 levels deep",
			"lllli1ei2ei3eeeee",
			3,
			[]int{1, 2, 3},
		},
	}

	for _, tc := range tt {
		r := NewBencodeReader(tc.value)
		value, err := DecodeList(r)
		if err != nil {
			t.Fatalf("failed to decode list %q: %v", tc.value, err)
		}

		list, ok := value.([]interface{})
		if !ok {
			t.Fatalf("failed to cast decoded value to list: %v", value)
		}

		for tc.nesting > 0 {
			list = list[0].([]interface{})
			tc.nesting--
		}

		if len(list) != len(tc.lastExpected) {
			t.Errorf("expected last nested list to have %d elements but got %d", len(tc.lastExpected), len(list))
		}

		for i, expected := range tc.lastExpected {
			a, ok := list[i].(int)
			if !ok {
				t.Errorf("expected item at index %d to be int from list", i)
			}

			if a != expected {
				t.Errorf("invalid value - expected %d got %d", tc.lastExpected[i], a)
			}

		}
	}
}

func TestDecodeDict(t *testing.T) {
}

func TestDecodeBencode(t *testing.T) {}
