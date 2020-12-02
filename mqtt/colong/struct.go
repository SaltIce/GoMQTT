package colong

import (
	"encoding/json"
	"Go-MQTT/mqtt/logger"
)

type MQTTComf struct {
	//节点ID
	Nid string `json:"nid"`
	//消息ID
	MsgID string `json:"msgID"`
}

//结构体编码,传实体过来
func JsonEncode(v interface{}) ([]byte, error) {
	if buf, err := json.Marshal(v); err != nil {
		logger.Error(err, "json marshal error:")
		return nil, err
	} else {
		return buf, nil
	}
}

//结构体解码，传地址过来
func JsonDecode(dst []byte, v interface{}) error {
	if err := json.Unmarshal(dst, v); err != nil {
		logger.Error(err, "json marshal error:")
		return err
	} else {
		return nil
	}
}
