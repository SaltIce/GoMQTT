package codec

import (
	"bytes"
	"encoding/binary"
)

const (
	// 包头魔术
	PacketHead uint32 = 0x123456
)

func PacketSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// 检查 atEOF 参数 和 数据包头部的四个字节是否 为 0x123456(我们定义的协议的魔数)
	if !atEOF && len(data) > 6 && binary.BigEndian.Uint32(data[:4]) == PacketHead {
		var l int16
		// 读取 数据包 实际数据的长度（大小为0~2^16）
		err = binary.Read(bytes.NewReader(data[4:6]), binary.BigEndian, &l)
		if err != nil {
			panic(err)
		}
		pl := int(l) + 6
		if pl <= len(data) {
			return pl, data[:pl], nil
		}
	}
	return
}

func Encode(data []byte) *bytes.Buffer {
	magicNum := make([]byte, 4)
	binary.BigEndian.PutUint32(magicNum, 0x123456)
	lenNum := make([]byte, 2)
	binary.BigEndian.PutUint16(lenNum, uint16(len(data)))
	packetBuf := bytes.NewBuffer(magicNum)
	packetBuf.Write(lenNum)
	packetBuf.Write(data)
	return packetBuf
}
