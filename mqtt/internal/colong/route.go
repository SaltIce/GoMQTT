package colong

import (
	"Go-MQTT/mqtt/internal/logger"
	"Go-MQTT/mqtt/internal/utils"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
)

//用来缓存需要向其它节点发送新增的主题数据的通道
var CacheTopicOut = utils.CacheTopicOut

//用来缓存从其它节点接收到的新增的主题数据的通道
var CacheTopicIn = make(chan Topics, 100)

//向其它节点发送与接收消息的缓冲区
var CacheMsgIn = utils.CacheMsgIn
var CacheMsgOut = utils.CacheMsgOut

var TopicAddTag = utils.TopicAddTag
var TopicDelTag = utils.TopicDelTag
var MQTTMsgAck = utils.MQTTMsgAck

type Topics struct {
	Topic string
	Tag   byte // 0x00增加，0x01删除
}
type MsgSt struct {
	Body  []byte `json:"Body"`
	Topic string `json:"Topic"`
	Id    string `json:"Id"`
}

//保留发送待确认的主题
/**
* 还要添加处理【超时未确认的，可以进行重发，待完成。。】
// FIXME:超时未确认的，可以进行重发，待完成。。
**/
type AwaitAck struct {
	// k:topic,v:待确认的节点id列表
	cm  map[string][]string
	rwm sync.RWMutex
}

//从一个topic等待确认的节点列表中删除一个节点devN
func (this *AwaitAck) del(topic, devN string) bool {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	v := this.cm[topic]
	for i, s := range v {
		if s == devN {
			this.cm[topic] = append(v[:i], v[i+1:]...)
			logger.Debugf("删除等待列表后：%v", this.cm[topic])
			return true
		}
	}
	return false
}

//添加等待确认的主题
func (this *AwaitAck) add(topic string, devN []string) bool {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	//判断是否存在当前topic对应的等待列表
	if _, ok := this.cm[topic]; ok {
		return false
	}
	this.cm[topic] = devN
	logger.Debugf("添加等待确认的主题:%v", this.cm)
	return true
}

type MsgAwaitAck struct {
	// k:消息packedId,v:待确认的节点id列表
	cm  map[string]*msgNode
	rwm sync.RWMutex
}
type msgNode struct {
	msg []byte
	dev []string
}

//从一个消息确认等待确认的节点列表中删除一个节点devN
func (this *MsgAwaitAck) del(id, devN string) bool {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	if v, ok := this.cm[id]; !ok {
		return false
	} else {
		vv := v.dev
		for i, s := range vv {
			if s == devN {
				this.cm[id].dev = append(vv[:i], vv[i+1:]...)
				logger.Debugf("%v删除等待后：%v", id, this.cm[id].dev)
				return true
			}
		}
		return false
	}

}

//添加等待确认的消息
func (this *MsgAwaitAck) add(id string, devN []string, msg []byte) bool {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	//判断是否存在当前topic对应的等待列表
	if _, ok := this.cm[id]; ok {
		this.cm[id].dev = append(this.cm[id].dev, devN...)
		return true
	}
	ms := msgNode{dev: devN, msg: msg}
	this.cm[id] = &ms
	logger.Debugf("添加等待确认的消息:%v", this.cm[id].dev)
	return true
}

//添加等待确认的消息,保存第一次确认的等待确认
func (this *MsgAwaitAck) add2(id string, devN string, msg []byte) bool {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	//判断是否存在当前topic对应的等待列表
	if _, ok := this.cm[id]; ok {
		return false
	}
	d := make([]string, 1)
	d[0] = devN
	ms := msgNode{dev: d, msg: msg}
	this.cm[id] = &ms
	logger.Debugf("添加保存第一次确认的等待确认的节点:%v", this.cm[id].dev)
	return true
}

type dev []string

//封装对外的路由表
type RouteTab struct {
	rwm   sync.RWMutex
	route route `json:"route"`
}

//路由表
type route struct {
	//key：主题，value：机器
	cm  map[string]dev `json:"cm"`
	rwm sync.RWMutex
}

//集群列表解码
func (this *RouteTab) Decode(b []byte) bool {
	this.route.rwm.RLock()
	defer this.route.rwm.RUnlock()
	if err := json.Unmarshal(b, &this.route.cm); err != nil {
		logger.Error(err, "json unmarshal error:")
		return false
	}
	return true
}

var cluster RouteTab

//主题消息确认等待表
var awaitAck AwaitAck

//mqtt消息发送端的确认等待表，第一次确认, 它的dev用来存等待确认的节点名称列表，它有多个数据
var msgAwaitAck MsgAwaitAck

//mqtt消息接收端的确认等待表，第二次确认, 它的dev用来存等待确认的节点名称，它只有一个数据
var msgInAwaitAck MsgAwaitAck

func init() {
	cluster.route.cm = make(map[string]dev)
	awaitAck.cm = make(map[string][]string)
	msgAwaitAck.cm = make(map[string]*msgNode)
	msgInAwaitAck.cm = make(map[string]*msgNode)
	//从CacheTopicOut中获取数据，并插入到本地路由表中，根据返回是true or false
	go autoUpdate()
	//从CacheMsgOut中获取消息，并发送出去
	go autoOutMsg()
}

//发送消息出去
func autoOutMsg() {
	for {
		select {
		case c := <-CacheMsgOut:
			var devs dev
			//获取还需要向那个节点发送数据的列表
			if len(c.Dev) > 0 {
				devs = c.Dev
			} else {
				devs = cluster.Get(c.MsgSt.Topic)
			}
			logger.Infof("等待发送的设备id：%v", devs)
			var bag dev
			if b, err := JsonEncode(c.MsgSt); err != nil {
				CacheMsgOut <- c
			} else {
				b1 := make([]byte, 1)
				b1[0] = MQTTMsgAck
				b = append(b1, b...)
				devN := make([]string, 1)
				for _, v := range devs {
					if v == MyNetNode.Net[0].Node.In.Name {
						//logger.Debug("无需发送自己")
						continue
					}
					devN = append(devN, v)
					cl := CL.get(v)
					//判断是否有会话
					if cl == nil {
						logger.ErrorNoStackInfof("当前%v没有会话，无法发送", v)
						continue
					}
					//发送出现错误，重新加入到缓冲队列里面
					if !cl.OutMsg(b) {
						bag = append(bag, v)
					} else {
						//应该再添加至等待队列里面，等待确认数据来
						logger.Info("添加至等待队列里面")
						msgAwaitAck.add(c.MsgSt.Id, devN, c.MsgSt.Body)
					}
				}
				if len(bag) > 0 {
					c.Dev = bag
					//发送存在发送失败，重新加入到缓冲队列里面
					CacheMsgOut <- c
					logger.Info("发送出现错误，重新加入到缓冲队列里面")
				}
			}

		}
	}
}

//从数组中删除一个
func delStrings(strs []string, str string) []string {
	if len(strs) <= 1 {
		return nil
	}
	for i, v := range strs {
		if v == str {
			return append(strs[:i], strs[i+1:]...)
		}
	}
	return nil
}

//从CacheTopicOut中获取数据，并插入到本地路由表中，根据返回是true or false
//true：通知其它节点更新
//false：不用通知其它节点
func autoUpdate() {
	for {
		select {
		case c := <-CacheTopicOut:
			if c.Tag == addAck {
				t := cluster.Insert(c.Topic, MyNetNode.Net[0].Node.In.Name)
				if t {
					AddTopic(c.Topic)
				}
			} else if c.Tag == delAck {
				t := cluster.Del(c.Topic, MyNetNode.Net[0].Node.In.Name)
				if t {
					DelTopic(c.Topic)
				}
			}
		}
	}
}

//获取对应的集群列表
func (this *RouteTab) Get(topic string) dev {
	//this.route.rwm.RLock()
	//defer this.route.rwm.RUnlock()
	return this.route.cm[topic]
}

const (
	EMPTY  = ""
	SINGLE = "+"
	MULTY  = "#"
	SYS    = "$"
)

//本地插入,主题和机器名称
//返回true表示要通知其它机器更新路由表
//返回false则表示，当前主题存在路由表里面，无需更新其它机器的路由表
func (this *RouteTab) Insert(topic string, devN string) bool {
	//topics := strings.Split(topic,"/")

	this.route.rwm.RLock()
	defer this.route.rwm.RUnlock()
	if value, ok := cluster.route.cm[topic]; ok {
		//主题存在,判断机器是否在里面
		for _, v := range value {
			if v == devN {
				logger.Debugf("当前设备在主题：%v里面", topic)
				return false
			}
		}
		value = append(value, devN)
		cluster.route.cm[topic] = value
	} else {
		//不存在则插入
		cluster.route.cm[topic] = dev{devN}
	}
	logger.Debugf("插入主题： %v ，设备： %v 到主题路由表里面", topic, devN)
	logger.Debugf("当前主题路由表：%v", cluster.route.cm)
	return true
}

//本地删除,主题和机器名称，不能删除不在主题路由表里的主题
//返回true表示存在且删除成功，false表示没有该机器在路由表里或者没有该主题
func (this *RouteTab) Del(topic string, devN string) bool {
	this.route.rwm.RLock()
	defer this.route.rwm.RUnlock()
	//判断是否有这个主题的信息
	if value, ok := cluster.route.cm[topic]; ok {
		//主题存在,判断机器是否在里面
		for i, v := range value {
			//在里面则移除它
			if v == devN {
				newV := append(value[:i], value[i+1:]...)
				cluster.route.cm[topic] = newV
				logger.Infof("机器在里面,已删除topic：%s列表中的%s", topic, devN)
				logger.Debugf("当前主题路由表：", cluster.route.cm)
				return true
			}
		}
	}
	logger.Debugf("当前主题路由表：", cluster.route.cm)
	return false
}

//对象序列化
func ExampleWrite(info interface{}) []byte {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.LittleEndian, info)
	if err != nil {
		fmt.Println("binary.Write failed:", err)
	}
	fmt.Printf("序列化消息对象：% x\n", buf.Bytes())
	return buf.Bytes()
}

func ExampleRead(b []byte, info interface{}) (bool, error) {
	buf := bytes.NewBuffer(b)

	err := binary.Read(buf, binary.LittleEndian, info)
	if err != nil {
		fmt.Println("binary.Read failed:", err)
		return false, err
	}
	return true, nil
	// Output: 3.141592653589793
}
