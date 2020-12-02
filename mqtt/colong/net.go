package colong

import (
	"bytes"
	"encoding/binary"
	uuid "github.com/satori/go.uuid"
	"Go-MQTT/mqtt/message"
	"Go-MQTT/mqtt/service"
	"Go-MQTT/mqtt/logger"

	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"net"
	"sync"
)

/**
* in 的只负责将当前节点需要从其它节点接收的消息并处理，不发送消息
*
**/
const (
	ackCon byte = iota
	ackComf
	ackPush
	getTabAck
	discAck            = 0x88 //服务发现表消息头
	infoAck            = 0x89 //通知各节点请求节点发现表
	mqttMsgAck    byte = 0x99
	mqttMsgOkAck  byte = 0x10 //mqtt消息的第一次确认码
	mqttMsgOk2Ack byte = 0x11 //mqtt消息的第二次确认码
	nodePingAck   byte = 0x80 //节点间互相ping的码
)
const (
	addAck byte = iota //添加topic
	delAck             //删除topic
)
const (
	conAck  byte = iota
	pingAck      //与节点发现服务器的ping心跳
)

type myNet struct {
	Net      []Net
	Discover disc
}
type disc struct {
	Addr string
}
type Net struct {
	DevId int
	Node  node
}
type node struct {
	In  nodes
	Out nodes
}
type nodes struct {
	Name string
	Port string
}

func ReadConstNet(t *myNet, filePath string) (*myNet, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		//logger.Error(err,"读取配置文件"+filePath+"出错")
		return nil, err
	}
	//把yaml形式的字符串解析成struct类型 t保存初始数据
	err = yaml.Unmarshal(data, t)
	if err != nil {
		//logger.Error(err,"解析配置文件"+filePath+"出错")
		return nil, err
	}
	return t, nil
	//d, _ := yaml.Marshal(&t)
	//fmt.Println("看看解析文件后的数据 :\n", string(d))
}

//节点的会话信息
type ClusterNode struct {
	NodeID    string   //连接id,通常为节点id
	Conn      net.Conn //连接信息
	Available bool     //是否可用
	Close     bool     //关闭标志
	//InBuf bytes.Buffer //输入缓冲区
	//OutBuf bytes.Buffer //输出缓冲区
}

//执行该会话复活操作
func (cluster *ClusterNode) Relive(conn net.Conn) {
	cluster.Available = true
	cluster.Close = false
	cluster.Conn = conn
}

//执行该会话死亡操作，应该在一段时间内不删除该会话，暂未去实现
func (cluster *ClusterNode) Death() {
	cluster.Available = false
	cluster.Close = true
	cluster.Conn.Close()
}

//发送给其它节点的Ping消息
func (cluster *ClusterNode) PingNode() {
	buf := make([]byte, 2)
	buf[0] = nodePingAck
	buf[1] = nodePingAck
	if _, err := cluster.Conn.Write(buf); err != nil {

	}
}

//发送更新主题确认消息的
func (cluster *ClusterNode) Acknowledgement(topic string) {
	buff := bytes.Buffer{}

	buf := make([]byte, 1)
	buf[0] = ackComf
	//写入ack码
	buff.Write(buf)
	//写入当前设备的节点id
	dn := []byte(MyNetNode.Net[0].Node.In.Name)
	dt := []byte(topic)
	//当前节点id长度
	buff.Write(Hextob(len(dn))[:1])
	//主题长度
	buff.Write(Hextob(len(dt))[:1])
	//节点id数据
	buff.Write(dn)
	//主题数据
	buff.Write(dt)
	cluster.Conn.Write(buff.Bytes())
}

//发送接收MQ消息的第一次确认
func (cluster *ClusterNode) AcknowledgementMQ(id string, devN string, msg []byte) {
	logger.Debugf("准备添加接收MQ消息：%v的第一次确认的等待确认", id)
	buff := bytes.Buffer{}

	buf := make([]byte, 1)
	buf[0] = mqttMsgOkAck
	//写入ack码
	buff.Write(buf)
	mq := MQTTComf{Nid: MyNetNode.Net[0].Node.In.Name, MsgID: id}
	if buf1, err := JsonEncode(mq); err != nil {
		return
	} else {
		buff.Write(buf1)
		go func() {
			j := 0
			ch := make(chan byte, 1)
			ch <- 0x00
			//最多循环5次
			for i := 0; i < 1 && j < 5; i++ {
				select {
				case <-ch:
					if _, err := cluster.Conn.Write(buff.Bytes()); err != nil {
						ch <- 0x00
					} else {
						logger.Debugf("发送第一次等待确认消息：%v", id)
						//添加到等待表中
						msgInAwaitAck.add2(id, devN, msg)
						i++
					}
					j++
				}
			}
			if j >= 5 {
				logger.ErrorNoStackInfof("发送第一次等待确认消息：%v------失败", id)
			}
		}()
	}
}

//int==>[]byte
func Hextob(n int) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.LittleEndian, int16(n))
	return bytebuf.Bytes() //[10,0],暂时只有前面第一个有用
}

//字节转换成整形
func BytesToInt(b byte) int {
	bb := make([]byte, 2)
	bb[0] = b
	bytesBuffer := bytes.NewBuffer(bb)

	var x int16
	j := 0
	for i := 0; i < 1 && j < 10; {
		err := binary.Read(bytesBuffer, binary.BigEndian, &x)
		if err == nil {
			i++
		}
		j++
	}
	return int(x)
}

var MyNetNode *myNet

func init() {
	co := myNet{}
	consts, err := ReadConstNet(&co, "mqtt/config/net.yml")
	if err != nil {
		panic(err)
	}
	MyNetNode = consts
	//默认一个输出端口
	if MyNetNode.Net[0].Node.In.Port == "" {
		MyNetNode.Net[0].Node.In.Port = "1884"
	}
	if MyNetNode.Net[0].Node.In.Name == "" {
		v := uuid.NewV4()
		MyNetNode.Net[0].Node.In.Name = v.String()
		MyNetNode.Net[0].Node.Out.Name = v.String()
	} else {
		MyNetNode.Net[0].Node.Out.Name = MyNetNode.Net[0].Node.In.Name
	}

	CS = clusterS{nodes: map[string]*ClusterNode{}}
}

//保存 节点名称->节点连接 的信息
type clusterS struct {
	nodes map[string]*ClusterNode
	rwm   sync.Mutex
}

//默认一个
var CS clusterS

func (cluster *clusterS) add(node string, c *ClusterNode) bool {
	cluster.rwm.Lock()
	defer cluster.rwm.Unlock()
	if _, ok := cluster.nodes[node]; ok {
		//存在则不能插入
		return false
	}
	cluster.nodes[node] = c
	return true
}
func (cluster *clusterS) update(node string, c *ClusterNode) bool {
	cluster.rwm.Lock()
	defer cluster.rwm.Unlock()
	if _, ok := cluster.nodes[node]; ok {
		//存在则直接更新
		cluster.nodes[node] = c
		return true
	}
	//不存在直接插入
	cluster.nodes[node] = c
	return true
}
func (cluster *clusterS) del(node string) {
	cluster.rwm.Lock()
	defer cluster.rwm.Unlock()
	if _, ok := cluster.nodes[node]; ok {
		delete(cluster.nodes, node)
	}
}
func (cluster *clusterS) get(node string) (bool, *ClusterNode) {
	if v, ok := cluster.nodes[node]; ok {
		return true, v
	} else {
		return false, nil
	}
}
func Nets() {
	// 创建用于监听的 socket
	for _, ok := range MyNetNode.Net {
		go func() {
			listener, err := net.Listen("tcp", "127.0.0.1:"+ok.Node.In.Port)
			if err != nil {
				logger.Errorf(err, "Listen %s\n", ok.Node.In.Name)
				return
			}
			logger.Infof("%s--监听套接字，创建成功。。。PORT：%s\n", ok.Node.In.Name, ok.Node.In.Port)
			// 服务器结束前关闭 listener
			defer listener.Close()

			for {
				// 创建用户数据通信的socket
				conn, err := listener.Accept() // 阻塞等待...
				if err != nil {
					logger.Error(err, "Accept err:")
					return
				}
				clusterNode := ClusterNode{NodeID: "", Conn: conn, Available: true, Close: false}
				go handleServer(&clusterNode)
			}
		}()
	}
}
func handleServer(clusterNode *ClusterNode) {
	defer clusterNode.Death()

	// 创建一个用保存数据的缓冲区
	buf := make([]byte, 4096)
	for {
		// 获取客户端发送的数据内容
		n, err := clusterNode.Conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				logger.ErrorNoStackInfof("有节点宕机了")
				return
			}
			logger.ErrorNoStackInfof("Read error,close connect:")
			return
		}
		// 处理 客户端 的数据
		hand(buf[:n], clusterNode)
		// 回写数据给客户端
		//_, err = clusterNode.Conn.Write([]byte("This is Server\n"))
		//if err != nil {
		//	fmt.Println("Write err:", err)
		//	return
		//}
	}
}
func hand(buf []byte, clusterNode *ClusterNode) {
	if len(buf) <= 3 {
		logger.ErrorNoStackInfof("获取字节数错误")
		return
	}
	tag := true
	//下面在节点1通知节点2有新增的主题时，节点1充当客户端，节点2充当服务端
	// 推送通知协议： 消息标志位（1bit）+ 数据域长度（客户端节点名称长度（1bit）+新增的主题长度（1bit））+ 数据域（名称 + 主题 ）+ 标志位// 暂时不加校验码
	// 连接消息协议： 标志位（1bit）+ 数据域长度（客户端节点名称长度（1bit））+ 数据域（名称）
	// 确认推送协议： 消息标志位（1bit）+ 数据域长度（服务端节点名称长度（1bit）+新增的主题长度（1bit））+ 数据域（名称 + 主题）
	switch buf[0] {
	case ackCon:
		tag = conHand(buf[1:], clusterNode)
	case ackComf:
		tag = comfHand(buf[1:], clusterNode)
		//通过返回的数据中，消息id去更新对应的消息状态
	case ackPush:
		tag = pushInHand(buf[1:], clusterNode)
	case mqttMsgAck:
		tag = pushInHandMQTT(buf[1:], clusterNode)
	case nodePingAck:
		tag = nodePing(buf, clusterNode)
	case mqttMsgOk2Ack:
		tag = mqttMsgOk02Handle(buf[1:], clusterNode)
	default:
		logger.Infof("从%v接收到异常数据：%v", clusterNode.NodeID, buf)
		tag = false
	}
	if !tag {
		//logger.ErrorNoStackInfof("节点%s，%s，被主动关闭", clusterNode.NodeID, clusterNode.Conn.RemoteAddr())
		//clusterNode.Death()
	}
}

//节点间的Ping的处理逻辑
func nodePing(buf []byte, clusterNode *ClusterNode) bool {
	lens := len(buf)
	//验证连接信息
	//如果数据域表示的长度和数据域后面的数据长度不相等 或者根本就没有数据，直接退出
	if lens != 2 || buf[0] != buf[1] {
		logger.ErrorNoStackInfof("节点%v发送的Ping数据错误", clusterNode.NodeID)
		return false
	}
	//clusterNode.PingNode()
	return true
}

//连接的处理逻辑
func conHand(buf []byte, clusterNode *ClusterNode) bool {
	lens := len(buf[1:])
	nameLen := int(buf[0])
	//验证连接信息
	//如果数据域表示的长度和数据域后面的数据长度不相等 或者根本就没有数据，直接退出
	if lens != nameLen || lens <= 0 {
		logger.ErrorNoStackInfof("当前连接数据格式错误")
		return false
	}
	name := string(buf[1 : nameLen+1])
	clusterNode.NodeID = name
	ok, c := CS.get(name)
	if ok {
		//如果存在之前的会话信息
		//更新保存到集群列表中去
		c.Relive(clusterNode.Conn)
		CS.update(name, c)
	} else {
		//不存在则插入,表示这是一个新来的节点
		CS.add(name, clusterNode)
	}
	logger.Debugf("%s建立连接会话成功", name)
	return true
}

//确认的ack消息
func comfHand(buf []byte, clusterNode *ClusterNode) bool {
	lens := len(buf[2:])
	nameLen := int(buf[0])
	topicLen := int(buf[1])
	//如果数据域表示的长度和数据域后面的数据长度不相等，直接退出
	if lens != (nameLen+topicLen) || lens <= 0 {
		logger.ErrorNoStackInfof("当前确认ack的数据格式错误")
		return false
	}
	name := string(buf[2 : nameLen+2])
	topic := string(buf[nameLen+2 : lens+2])
	//删除等待列表中的数据
	awaitAck.del(topic, name)
	logger.Debugf("节点%s对主题%s的确认成功", name, topic)
	return true
}

//被推送新主题数据上来的处理逻辑
func pushInHand(buf []byte, clusterNode *ClusterNode) bool {
	lens := len(buf[2:])
	nameLen := int(buf[0])
	topicLen := int(buf[1])
	//如果数据域表示的长度和数据域后面的数据长度不相等，直接退出
	if lens != (nameLen+topicLen+1) || lens <= 0 {
		logger.ErrorNoStackInfof("当前推送上来的数据格式错误")
		return false
	}
	name := string(buf[2 : nameLen+2])
	topic := string(buf[nameLen+2 : lens+1])
	tag := buf[lens+1]
	isAdd := false
	switch tag {
	case addAck: //添加topic
		isAdd = true
	case delAck: //删除
	default: //未知的
		return false
	}
	//判断当前节点是否存在集群列表中，没有代表之前没有创建连接
	if ok, v := CS.get(name); ok {
		v.Relive(clusterNode.Conn)
		s := ""
		//更新主题路由表
		if isAdd {
			s = "添加"
			//本地路由表添加
			cluster.Insert(topic, name)
		} else {
			s = "删除"
			//本地路由表删除
			cluster.Del(topic, name)
		}
		//发送确认消息
		v.Acknowledgement(topic)
		logger.Debugf("节点%s推送的新主题%s%s成功", name, topic, s)
		return true
	} else {
		//当前节点未发送连接消息
		return false
	}
}

//负责处理接收到第二次mqtt消息确认的处理，要删除MQTT消息第二次确认表中的数据
func mqttMsgOk02Handle(buf []byte, clusterNode *ClusterNode) bool {
	mq := MQTTComf{}
	if err := JsonDecode(buf, &mq); err != nil {
		return false
	} else {
		msgInAwaitAck.del(mq.MsgID, mq.Nid)
		return true
	}
}

//被推送mqtt数据上来的处理逻辑
func pushInHandMQTT(buf []byte, clusterNode *ClusterNode) bool {
	msg := MsgSt{}
	err1 := JsonDecode(buf, &msg)
	if err1 != nil {
		logger.Error(err1, "mqtt数据解析错误")
	}
	buf = msg.Body
	if len(buf)<2{
		logger.Infof("当前接收的数据错误。。。{}",buf)
		return false
	}
	lens := len(buf[2:])
	msgLen := binary.LittleEndian.Uint16(buf[0:2])
	logger.Debugf("被推送mqtt数据上来的处理逻辑: %v，%v", int(msgLen), lens)
	//如果数据域表示的长度和数据域后面的数据长度不相等，直接退出
	if lens != int(msgLen) || lens <= 0 {
		logger.ErrorNoStackInfof("当前推送上来的MQTT数据格式错误")
		return false
	}
	ms := message.PublishMessage{}
	ms.SetType(message.PUBLISH)
	_, err := ms.Decode(buf[2:])
	//_, err := ExampleRead(buf[2:], &ms)
	if err != nil {
		logger.Error(err, "消息解码错误")
		return true
	}
	topic := string(ms.Topic())
	//发送第一次确认MQ消息
	clusterNode.AcknowledgementMQ(msg.Id, clusterNode.NodeID, buf[2:])
	er := service.SVC.ProcessIncoming02(&ms)
	if er != nil {
		logger.Error(er, "接收到其它节点的消息，在推送数据过程中出现错误")
	}
	logger.Debugf("节点%s推送的主题‘%s’消息成功：……>> %v", clusterNode.NodeID, topic, ms.String())
	return true

}
