package btencoding

import (
	"bytes"
	"fmt"
	"sort"
)

type BenEncoder struct {
	buf *bytes.Buffer
}

func NewBenEncoder() *BenEncoder {
	return &BenEncoder{
		buf: bytes.NewBuffer(nil),
	}
}

func (b *BenEncoder) encode(value interface{}) ([]byte, error) {

	switch v := value.(type) {
	case string:
		fmt.Fprintf(b.buf, "%d:%s", len(v), v)
	case int, int8, int16, int32, int64:
		fmt.Fprintf(b.buf, "i%de", v)
	case []interface{}:
		b.encodeList(v)
	case map[string]interface{}:
		b.encodeDict(v)
	default:
		return nil, fmt.Errorf("unknown type for encoding: %T", v)
	}

	return b.buf.Bytes(), nil
}

func (b *BenEncoder) encodeList(list []interface{}) {
	fmt.Fprintf(b.buf, "l")
	for _, i := range list {
		b.encode(i)
	}
	fmt.Fprintf(b.buf, "e")
}

func (b *BenEncoder) encodeDict(dict map[string]interface{}) {
	// bencoding requries keys to be lexographically sorted
	fmt.Fprintf(b.buf, "d")
	keys := []string{}
	for k := range dict {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		if k != "" {
			b.encode(k)
		} else {
			continue
		}
		if v, ok := dict[k]; ok {
			b.encode(v)
		}
	}
	fmt.Fprintf(b.buf, "e")
}
