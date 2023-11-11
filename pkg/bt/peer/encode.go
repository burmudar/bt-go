package peer

import "encoding/binary"

func EncodeMessage(m Message) []byte {
	tag := m.Tag()
	payload := m.Payload()
	data := make([]byte, 4+1+len(payload))

	msgLen := len(data) - 4
	binary.BigEndian.PutUint32(data, uint32(msgLen))
	data[4] = byte(tag)
	if len(payload) > 0 {
		copy(data[5:], payload)
	}
	return data
}
