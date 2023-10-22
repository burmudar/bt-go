package main

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
		b.encodeString(v)
	case int:
		b.encodeInt(v)
	case []interface{}:
		b.encodeList(v)
	case map[string]interface{}:
		b.encodeDict(v)
	}

	return b.buf.Bytes(), nil
}

func (b *BenEncoder) encodeInt(v int) {
	fmt.Fprintf(b.buf, "i%de", v)
}

func (b *BenEncoder) encodeString(v string) {
	fmt.Fprintf(b.buf, "%d:%s", len(v), v)
}

func (b *BenEncoder) encodeList(list []interface{}) {
	fmt.Fprintf(b.buf, "l")
	for _, i := range list {
		b.encode(i)
	}
	fmt.Fprintf(b.buf, "e")
}

func (b *BenEncoder) encodeDict(dict map[string]interface{}) {
	fmt.Fprintf(b.buf, "l")
	// bencoding requries keys to be lexographically sorted
	keys := []string{}
	for k := range dict {
		keys = append(keys, k)
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i] >= keys[j]
	})
	for _, k := range keys {
		b.encode(k)
		b.encode(dict[k])
	}
	fmt.Fprintf(b.buf, "e")
}
