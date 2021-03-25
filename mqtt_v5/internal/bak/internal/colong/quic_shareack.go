// 请求共享消息
package colong

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync/atomic"
)

// ShareReq消息的确认，返回当前节点是否可以接收这批主题的share竞争，已经竞争权重
type ShareAckMessage struct {
	header

	topics [][]byte
	weight [][]byte //各个主题的权重，如果当前主题没有对应的参与共享者，则为[]byte{0x00}即可
}

var _ Message = (*ShareAckMessage)(nil)

// NewShareAckMessage creates a new ShareAck message.
func NewShareAckMessage() *ShareAckMessage {
	msg := &ShareAckMessage{}
	msg.SetType(SHAREACK)

	return msg
}

func (this ShareAckMessage) String() string {
	msgstr := fmt.Sprintf("%s, Packet ID=%d", this.header, this.PacketId())

	for i, t := range this.topics {
		msgstr = fmt.Sprintf("%s, Weight[%d]=%q/%d", msgstr, i, string(t), this.weight[i])
	}

	return msgstr
}

// Topics returns a list of topics sent by the Client.
func (this *ShareAckMessage) Topics() [][]byte {
	return this.topics
}

// AddTopic adds a single topic to the message, along with the corresponding QoS.
// An error is returned if QoS is invalid.
func (this *ShareAckMessage) AddTopic(topic []byte, weight uint64) error {

	var i int
	var t []byte
	var found bool

	for i, t = range this.topics {
		if bytes.Equal(t, topic) {
			found = true
			break
		}
	}

	if found {
		v := make([]byte, 8)
		i = binary.PutUvarint(v, weight)
		this.weight[i] = v[:i]
		return nil
	}

	this.topics = append(this.topics, topic)
	v := make([]byte, 8)
	i = binary.PutUvarint(v, weight)
	this.weight = append(this.weight, v[:i])
	this.dirty = true

	return nil
}

// RemoveTopic removes a single topic from the list of existing ones in the message.
// If topic does not exist it just does nothing.
func (this *ShareAckMessage) RemoveTopic(topic []byte) {
	var i int
	var t []byte
	var found bool

	for i, t = range this.topics {
		if bytes.Equal(t, topic) {
			found = true
			break
		}
	}

	if found {
		this.topics = append(this.topics[:i], this.topics[i+1:]...)
		this.weight = append(this.weight[:i], this.weight[i+1:]...)
	}

	this.dirty = true
}

// TopicExists checks to see if a topic exists in the list.
func (this *ShareAckMessage) TopicExists(topic []byte) bool {
	for _, t := range this.topics {
		if bytes.Equal(t, topic) {
			return true
		}
	}

	return false
}

// TopicQos returns the QoS level of a topic. If topic does not exist, QosFailure
// is returned.
func (this *ShareAckMessage) TopicWeight(topic []byte) uint64 {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			v, _ := binary.Uvarint(this.weight[i])
			return v
		}
	}

	return WeightFailure
}

// Qos returns the list of QoS current in the message.
func (this *ShareAckMessage) Weight() []uint64 {
	ret := make([]uint64, 0)
	for _, v := range this.weight {
		vi, _ := binary.Uvarint(v)
		ret = append(ret, vi)
	}
	return ret
}

func (this *ShareAckMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

func (this *ShareAckMessage) Decode(src []byte) (int, error) {
	total := 0

	hn, err := this.header.decode(src[total:])
	total += hn
	if err != nil {
		return total, err
	}

	//this.packetId = binary.BigEndian.Uint16(src[total:])
	this.packetId = src[total : total+2]
	total += 2

	remlen := int(this.remlen) - (total - hn)
	for remlen > 0 {
		t, n, err := readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		remlen = remlen - n
		this.topics = append(this.topics, t)
		// 权重
		t, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		this.weight = append(this.weight, t)

		remlen = remlen - n
	}

	if len(this.topics) == 0 {
		return 0, fmt.Errorf("shareack/Decode: Empty topic list")
	}

	this.dirty = false

	return total, nil
}

func (this *ShareAckMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("shareack/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	hl := this.header.msglen()
	ml := this.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("shareack/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0, err
	}

	total := 0

	n, err := this.header.encode(dst[total:])
	total += n
	if err != nil {
		return total, err
	}

	if this.PacketId() == 0 {
		this.SetPacketId(uint16(atomic.AddUint64(&gPacketId, 1) & 0xffff))
		//this.packetId = uint16(atomic.AddUint64(&gPacketId, 1) & 0xffff)
	}

	n = copy(dst[total:], this.packetId)
	//binary.BigEndian.PutUint16(dst[total:], this.packetId)
	total += n

	for i, t := range this.topics {
		n, err := writeLPBytes(dst[total:], t)
		total += n
		if err != nil {
			return total, err
		}
		// 权重
		n, err = writeLPBytes(dst[total:], this.weight[i])
		total += n
		if err != nil {
			return total, err
		}
	}

	return total, nil
}

func (this *ShareAckMessage) msglen() int {
	// packet ID
	total := 2

	for _, t := range this.topics {
		total += 2 + len(t) + 1
	}

	return total
}
