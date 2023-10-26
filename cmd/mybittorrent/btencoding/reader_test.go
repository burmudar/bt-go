package btencoding

import (
	"fmt"
	"testing"
)

func TestReadInt(t *testing.T) {
	isNilErr := func(err error) (bool, string) {
		return err == nil, "expected err to be nil"
	}
	tt := []struct {
		value    string
		expected int
		errFn    func(err error) (bool, string)
	}{
		{
			"i32e",
			32,
			isNilErr,
		},
		{
			"i10000e",
			10000,
			isNilErr,
		},
		{
			"i-10000e",
			-10000,
			isNilErr,
		},
		{
			"ie",
			0,
			func(err error) (bool, string) {
				return err != nil, "expected err to be not nil"
			},
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("ReadInt of %s", tc.value), func(t *testing.T) {
			r := NewBencodeReader(tc.value)

			num, err := r.ReadInt()

			if ok, reason := tc.errFn(err); !ok {
				t.Errorf("err assertion failed - %s: %v", reason, err)
			}

			if num != tc.expected {
				t.Errorf("%s: expected %d got %d", tc.value, tc.expected, num)
			}
		})
	}
}

func TestReadString(t *testing.T) {
	isNilErr := func(err error) (bool, string) {
		return err == nil, "expected err to be nil"
	}
	tt := []struct {
		value    string
		expected string
		errFn    func(err error) (bool, string)
	}{
		{
			"7:william",
			"william",
			isNilErr,
		},
		{
			"1:w",
			"w",
			isNilErr,
		},
		{
			"0:william",
			"",
			isNilErr,
		},
		{
			"8:short",
			"",
			func(err error) (bool, string) {
				return err != nil, "expected err for short string"
			},
		},
		{
			"no-colon",
			"",
			func(err error) (bool, string) {
				return err != nil, "expected err for string without colon"
			},
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("ReadString of %s", tc.value), func(t *testing.T) {
			r := NewBencodeReader(tc.value)

			str, err := r.ReadString()

			if ok, reason := tc.errFn(err); !ok {
				t.Fatalf("err assertion failed - %s: %v", reason, err)
			}

			if str != tc.expected {
				t.Errorf("%q: expected %q got %q", tc.value, tc.expected, str)
			}
		})
	}
}
