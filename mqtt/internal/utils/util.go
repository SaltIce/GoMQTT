package utils

import (
	"Go-MQTT/mqtt/internal/config"
	"Go-MQTT/mqtt/internal/logger"
	"Go-MQTT/mqtt/internal/message"
	"bytes"
	"encoding/binary"
	uuid "github.com/satori/go.uuid"
)

const (
	addAck byte = iota //添加topic
	delAck             //删除topic
)

var TopicAddTag = addAck
var TopicDelTag = delAck
var MQTTMsgAck byte = 0x99

type MsgSt struct {
	Body  []byte `json:"Body"`
	Topic string `json:"Topic"`
	Id    string `json:"Id"`
}
type MsgStWarp struct {
	MsgSt MsgSt
	Dev   []string
}
type Topics struct {
	Topic string
	Tag   byte // 0x00增加，0x01删除
}

//用来缓存需要向其它节点发送新增的主题数据的通道
var CacheTopicOut = make(chan Topics, 100)

//用来缓存从其它节点接收到的新增的主题数据的通道
var CacheTopicIn = make(chan Topics, 100)

//向其它节点发送与接收消息的缓冲区
var CacheMsgIn = make(chan MsgSt, 200)
var CacheMsgOut = make(chan MsgStWarp, 200)

//转发mqtt publishMessage给其它节点
func WriteMqtt(msg *message.PublishMessage) {
	logger.Debug(">>>>>>>>>>>>>>>" + msg.String())
	//这里编码消息并通过out口发送出去
	b := make([]byte, msg.Len())
	_, err := msg.Encode(b)
	if err != nil {
		logger.Error(err, "消息编码错误")
	}
	lens := Hextob(len(b))
	bb := bytes.Buffer{}
	bb.Write(lens)
	bb.Write(b)

	id := <-UID
	msgSt := MsgSt{Topic: string(msg.Topic()), Body: bb.Bytes(), Id: id}
	msgStWarp := MsgStWarp{MsgSt: msgSt}
	//发送至缓冲区
	CacheMsgOut <- msgStWarp
}

//uuid生成缓存
var UID chan string

var ClusterEnabled bool = false

func init() {
	co := config.MyConst{}
	consts, err := config.ReadConst(&co, "mqtt/config/const.yml")
	if err != nil {
		panic(err)
	}
	ClusterEnabled = consts.Cluster.Enabled
	if ClusterEnabled {
		UID = make(chan string, 100)
		go func() {
			//持续生出uuid
			for {
				v := uuid.NewV4()
				select {
				case UID <- v.String():
				}
			}
		}()
	}
}
func Hextob(n int) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.LittleEndian, int16(n))
	return bytebuf.Bytes() //[10,0],暂时只有前面第一个有用
}
