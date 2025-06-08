package protocol

import (
	"bufio"
	"encoding/binary"
	"io"
)

const (
	// HeaderLength 定义消息头的大小
	HeaderLength = 4
)

// Encode 打包消息为“长度+内容”的格式
func Encode(payload []byte) ([]byte, error) {
	length := int32(len(payload))
	frame := make([]byte, HeaderLength+length)
	binary.BigEndian.PutUint32(frame[:HeaderLength], uint32(length))
	copy(frame[HeaderLength:], payload)
	return frame, nil
}

// Decode 解包消息，返回内容和剩余数据
func Decode(reader *bufio.Reader) ([]byte, error) {
	header := make([]byte, HeaderLength)
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header)

	payload := make([]byte, length)
	_, err = io.ReadFull(reader, payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
