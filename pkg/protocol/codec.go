package protocol

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
)

const (
	// HeaderLength 定义消息头的大小
	HeaderLength = 4
)

// Message 将一个Message对象编码成数据帧
func EncodeMessage(msg Message) ([]byte, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return encodeFrame(payload)
}

// DecodeMessage 从数据帧中解码出一个Message对象
func DecodeMessage(reader *bufio.Reader) (*Message, error) {
	var msg Message
	payload, err := decodeFrame(reader)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(payload, &msg)
	return &msg, err
}

// encodeFrame 打包消息为“长度+内容”的格式
func encodeFrame(payload []byte) ([]byte, error) {
	length := int32(len(payload))
	frame := make([]byte, HeaderLength+length)
	binary.BigEndian.PutUint32(frame[:HeaderLength], uint32(length))
	copy(frame[HeaderLength:], payload)
	return frame, nil
}

// decodeFrame 解包消息，返回内容和剩余数据
func decodeFrame(reader *bufio.Reader) ([]byte, error) {
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
