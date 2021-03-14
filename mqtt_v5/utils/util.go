package utils

const (
	addAck byte = iota //添加topic
	delAck             //删除topic
)

var TopicAddTag = addAck
var TopicDelTag = delAck
var MQTTMsgAck byte = 0x99
