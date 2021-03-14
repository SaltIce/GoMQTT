package main

import (
	"Go-MQTT/mqtt/internal/logger"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

/**
*
*  节点发现服务器：
*		当节点连接数达到了一定数目后，发送一次提醒获取节点发现表消息
*		此时各节点请求节点发现表，完成节点之间的发现与连接。
*		之后，当有新的节点连接，或者节点断开，会进行通知各节点
*
**/
const (
	conAck byte = iota
	pingAck
	//主题推送
	ackPush
	//获取主题
	getTab
	getAck  = 0x88
	discAck = 0x88 //服务发现表消息头
	infoAck = 0x89 //通知各节点请求节点发现表
)
const (
	ackCon byte = iota
	ackComf
)
const (
	addAck byte = iota //添加topic
	delAck             //删除topic
)

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

//默认路由表
var clusters RouteTab

//获取对应的集群列表
func (this *RouteTab) Get(topic string) dev {
	this.route.rwm.RLock()
	defer this.route.rwm.RUnlock()
	return this.route.cm[topic]
}

//集群列表编码
func (this *RouteTab) Encode() []byte {
	this.route.rwm.RLock()
	defer this.route.rwm.RUnlock()
	if buf, err := json.Marshal(this.route.cm); err != nil {
		logger.Error(err, "json marshal error:")
		return nil
	} else {
		return buf
	}
}

//本地插入,主题和机器名称
//返回true表示要通知其它机器更新路由表
//返回false则表示，当前主题存在路由表里面，无需更新其它机器的路由表
func (this *RouteTab) Insert(topic string, devN string) bool {
	this.route.rwm.RLock()
	defer this.route.rwm.RUnlock()
	if value, ok := clusters.route.cm[topic]; ok {
		//主题存在,判断机器是否在里面
		for _, v := range value {
			if v == devN {
				logger.Debugf("当前设备在主题：%v里面", topic)
				return false
			}
		}
		value = append(value, devN)
		clusters.route.cm[topic] = value
	} else {
		//不存在则插入
		clusters.route.cm[topic] = dev{devN}
	}
	logger.Debugf("插入主题： %v ，设备： %v 到主题路由表里面", topic, devN)
	logger.Debugf("当前主题路由表：", clusters.route.cm)
	return true
}

//本地删除,主题和机器名称，不能删除不在主题路由表里的主题
//返回true表示存在且删除成功，false表示没有该机器在路由表里或者没有该主题
func (this *RouteTab) Del(topic string, devN string) bool {
	this.route.rwm.RLock()
	defer this.route.rwm.RUnlock()
	//判断是否有这个主题的信息
	if value, ok := clusters.route.cm[topic]; ok {
		//主题存在,判断机器是否在里面
		for i, v := range value {
			//在里面则移除它
			if v == devN {
				newV := append(value[:i], value[i+1:]...)
				clusters.route.cm[topic] = newV
				logger.Infof("机器在里面,已删除topic：%s列表中的%s", topic, devN)
				logger.Debugf("当前主题路由表：", clusters.route.cm)
				return true
			}
		}
	}
	logger.Debugf("当前主题路由表：", clusters.route.cm)
	return false
}

//节点集合
type CLNode struct {
	CL  map[string]*ClusterNode
	rwm sync.Mutex
}

//通知各节点请求节点发现表
// 0x89 + 版本号
func (this *CLNode) info() {
	bb := bytes.Buffer{}
	b1 := make([]byte, 2)
	b1[0] = infoAck
	b1[1] = Hextob(DisCoverTabs.version)[0]
	bb.Write(b1)
	this.rwm.Lock()
	defer this.rwm.Unlock()
	for _, v := range this.CL {
		v.Conn.Write(bb.Bytes())
	}
	logger.Info("开始通知各节点开始请求节点发现表")
}

//获得对应的
func (this *CLNode) get(id string) *ClusterNode {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	return this.CL[id]
}

//添加一个
func (this *CLNode) update(id string, clusterNode *ClusterNode) {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	this.CL[id] = clusterNode
}

//删除一个
func (this *CLNode) delete(id string) {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	logger.Debugf("删除节点：%v，会话信息", id)
	delete(this.CL, id)
	//通知更新节点发现表
	infoUpdate <- 0x00
}

var pingTime = 1 * time.Minute //超时时间 一分钟
var pingNums = 5               //最多5次未收到消息
//节点的会话信息
type ClusterNode struct {
	NodeID    string   //连接id,通常为节点id
	Conn      net.Conn //连接信息
	Available bool     //是否可用
	Close     bool     //关闭标志
	timeS     int64    //时间戳
	pingNum   int      //每分钟未发送消息，则记录一次，当到达5次时，判定断开连接
	//InBuf bytes.Buffer //输入缓冲区
	//OutBuf bytes.Buffer //输出缓冲区
}

//发送更新主题确认消息的
func (cluster *ClusterNode) Acknowledgement(topic string) {
	buff := bytes.Buffer{}

	buf := make([]byte, 1)
	buf[0] = ackComf
	//写入ack码
	buff.Write(buf)
	//写入当前设备的节点id
	dn := []byte("nodeDiscoverServer")
	dt := []byte(topic)
	//当前节点id长度
	buff.Write(Hextob(len(dn))[:1])
	//主题长度
	buff.Write(Hextob(len(dt))[:1])
	//节点id数据
	buff.Write(dn)
	//主题数据
	buff.Write(dt)

	j := 0
	for i := 0; i < 1 && j < 5; {
		//发送出去
		if _, err := cluster.Conn.Write(buff.Bytes()); err == nil {
			i++
			break
		}
		j++
	}
}

//发送路由表消息
func (cluster *ClusterNode) tabPushC() {
	buff := bytes.Buffer{}

	buf := make([]byte, 1)
	buf[0] = getTab
	//写入ack码
	buff.Write(buf)
	b := clusters.Encode()
	logger.Debugf("推送路由表数据：%v", b)
	if b == nil {
		return
	}
	//表长度
	buff.Write(Hextob(len(b)))
	//表数据
	buff.Write(b)

	j := 0
	for i := 0; i < 1 && j < 5; {
		//发送出去
		if _, err := cluster.Conn.Write(buff.Bytes()); err == nil {
			i++
			logger.Debug("推送路由表成功")
			break
		}
		j++
	}
}
func getTimeSenc() int64 {
	return time.Now().Unix() //单位秒
}

//执行该会话复活操作
func (cluster *ClusterNode) Relive(conn net.Conn) {
	cluster.Available = true
	cluster.Close = false
	cluster.Conn = conn
	cluster.pingNum = 0
	cluster.timeS = getTimeSenc()
}

//检查时间戳，用来
func (cluster *ClusterNode) CheckTime() {
	if cluster.pingNum >= pingNums {
		cluster.Death()
	}
	if time.Now().Sub(time.Unix(cluster.timeS, 0)) > pingTime {
		cluster.pingNum++
	}
}

//执行该会话死亡操作，应该在一段时间内不删除该会话，暂未去实现
func (cluster *ClusterNode) Death() {
	cluster.Available = false
	cluster.Close = true
	cluster.pingNum = 0
	cluster.Conn.Close()
	//删除发现表对应的
	DisCoverTabs.delDisc(cluster.NodeID)
	//客户端的会话的集合中对应的
	CL.delete(cluster.NodeID)
}

type discoverTab struct {
	nodes   map[string]*node //节点列表 sid:node
	rwm     sync.Mutex
	version int //当前发现表的版本号,只要有更新就会变化即加一，当到达127时就会变为0，主要是因为下面的字节解析用了int8，为了转换时只要一字节就行
}
type node struct {
	sid  string
	addr string
	//-1代表死亡
	version int //当前节点的版本号,只要有更新就会变化即加一，当到达127时就会变为0，主要是因为下面的字节解析用了int8，为了转换时只要一字节就行
}

//更新操作
func (this *discoverTab) update(id string, node *node) {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	no := this.nodes[id]
	if no == nil {
		//没有这个节点，就直接添加
		this.nodes[id] = node
	} else {
		if no.version == 127 {
			no.version = 0
		} else {
			no.version++
		}
		no.addr = node.addr
		no.sid = node.sid
	}
	if this.version == 127 {
		this.version = 0
	} else {
		this.version++
	}
	logger.Infof("当前发现表：%v", this.nodes)
}

//删除发现表中的一个
func (this *discoverTab) delDisc(id string) {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	if _, ok := this.nodes[id]; ok {
		logger.Infof("删除节点：%s发现数据", id)
		delete(this.nodes, id)
		logger.Infof("当前发现表：%v", this.nodes)
	}

}

//当前最新的节点发现表
var DisCoverTabs discoverTab

//阈值
var num int

//当前服务的端口
var port string

//保存客户端的会话的集合
var CL CLNode

//否则通知节点更新节点发现表的标志管道
var infoUpdate chan byte

func init() {
	//获取用户设置的节点数
	num = 2
	//获取该服务的端口
	port = "1890"
	DisCoverTabs.nodes = make(map[string]*node)
	CL.CL = make(map[string]*ClusterNode)
	infoUpdate = make(chan byte, 1)
	clusters.route.cm = make(map[string]dev)
}

//节点发现服务器
//当连接的节点到达设置的节点数时，就会向所有节点发送一个节点发现表
func main() {
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:"+port)
		if err != nil {
			logger.Error(err, "Listen error: %s\n")
			return
		}
		logger.Infof("监听套接字，创建成功。。。PORT：%s\n", port)
		// 服务器结束前关闭 listener
		defer listener.Close()
		go updateInfo()
		go info()
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
	for {
		time.Sleep(1 * time.Second)
	}
}

//用来在系统启动后的第一次通知各节点需要更新节点发现表了，将向infoUpdate管道发送一个数据，使得info()解除阻塞
func updateInfo() {
	for i := 0; i < 1; {
		if len(CL.CL) == num {
			//当系统下的注册节点达到一定数目时
			infoUpdate <- 0x00
			//并将num至为-1，这样标志着，之后的每次节点连接、断开都会通知其它节点
			num = -1
			i++
		}
		time.Sleep(time.Second * 1)
	}
}

//通知各节点请求节点发现表
func info() {
	for {
		select {
		case <-infoUpdate:
			CL.info()
		}
		//休眠一秒
		time.Sleep(time.Second * 1)
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
		hand(buf[:n], clusterNode)
	}
}
func hand(buf []byte, clusterNode *ClusterNode) chan bool {
	ch := make(chan bool, 1)
	if len(buf) <= 1 {
		logger.ErrorNoStackInfof("获取字节数错误")
		ch <- false
		return ch
	}
	tag := true
	//下面在节点1通知节点2有新增的主题时，节点1充当客户端，节点2充当服务端
	// 0x01 获取节点发现表协议： 消息标志位（1bit）+ 数据域长度（客户端节点名称长度（1bit）+1）+ 数据域（名称 + 版本号（1bit） ）// 暂时不加校验码
	// 0x88 连接消息协议： 标志位（1bit）+ 数据域长度（客户端节点名称长度（1bit）+端口长度（1bit））+ 数据域（名称+port）
	switch buf[0] {
	case conAck:
		tag = conHand(buf[1:], clusterNode)
		if tag && num == -1 {
			//通知info()进行通知
			infoUpdate <- 0x00
		}
		//更新时间戳
		clusterNode.timeS = getTimeSenc()
	case getAck:
		tag = getHand(buf[1:])
		//更新时间戳
		clusterNode.timeS = getTimeSenc()
	case pingAck:
		tag = pingHandle(*clusterNode, buf)
		//更新时间戳
		clusterNode.timeS = getTimeSenc()
	case ackPush:
		tag = pushInHand(buf[1:], clusterNode)
	case getTab:
		tag = tagPush(buf, clusterNode)
	default:
		logger.Infof("从 %v 接收到异常数据，断开连接：%v", clusterNode.NodeID, clusterNode.Conn.RemoteAddr())
		tag = false
	}
	if !tag {
		logger.ErrorNoStackInfof("节点%s，%s，被主动关闭", clusterNode.NodeID, clusterNode.Conn.RemoteAddr())
		clusterNode.Death()
	}
	ch <- tag
	return ch
}

//请求获取路由表的数据
func tagPush(buf []byte, clusterNode *ClusterNode) bool {
	logger.Debugf("节点%v请求路由表", clusterNode.NodeID)
	//简单的判断，这个的tagPush消息：两个getTab
	if len(buf) != 2 {
		return false
	}
	if buf[0] == buf[1] {
		clusterNode.tabPushC()

		return true
	}
	return false
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
	if v := CL.get(name); v != nil {
		v.Relive(clusterNode.Conn)
		s := ""
		//更新主题路由表
		if isAdd {
			s = "添加"
			//本地路由表添加
			clusters.Insert(topic, name)
		} else {
			s = "删除"
			//本地路由表删除
			clusters.Del(topic, name)
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

//处理ping 心跳消息
func pingHandle(clusterNode ClusterNode, pingMsg []byte) bool {
	//简单的判断，这个的ping消息：两个pingAck
	if len(pingMsg) != 2 {
		return false
	}
	if pingMsg[0] == pingMsg[1] {
		//简单一点的，直接再写回去
		_, err := clusterNode.Conn.Write(pingMsg)
		if err != nil {
			logger.Errorf(err, "节点%v，PING 响应消息错误", clusterNode.NodeID)
		}
		logger.Debugf("接收到节点%v，PING消息：%v", clusterNode.NodeID, clusterNode.Conn.RemoteAddr())
		return true
	}
	return false
}

//连接的处理逻辑，获取id与port
// 0x00 连接消息协议： 标志位（1bit）+ 数据域长度（客户端节点名称长度（1bit）+端口长度（1bit））+ 数据域（名称+port）
func conHand(buf []byte, clusterNode *ClusterNode) bool {
	lens := len(buf[2:])
	nameLen := int(buf[0])
	portLen := int(buf[1])
	//验证连接信息
	//如果数据域表示的长度和数据域后面的数据长度不相等 或者根本就没有数据，直接退出
	if lens != nameLen+portLen || lens <= 0 {
		logger.ErrorNoStackInfof("当前连接数据格式错误")
		return false
	}
	name := string(buf[2 : nameLen+2])
	ports := string(buf[nameLen+2 : portLen+nameLen+2])
	addr := strings.Split(clusterNode.Conn.RemoteAddr().String(), ":")[0] + ":" + ports
	logger.Infof("接收到节点：%v,addr：%v", name, addr)
	clusterNode.NodeID = name
	//保存会话
	CL.update(name, clusterNode)

	//添加到节点发现表里面
	DisCoverTabs.update(name, &node{sid: name, addr: addr, version: 0})

	logger.Debugf("%s建立连接会话成功", name)
	return true
}

// 0x88 获取节点发现表协议： 消息标志位（1bit）+ 数据域长度（客户端节点名称长度+1）（1bit）+ 数据域（名称 + 版本号（1bit） ）// 暂时不加校验码
func getHand(buf []byte) bool {
	lens := len(buf[1:])
	nameLen := int(buf[0]) - 1
	if lens != nameLen+1 || lens <= 0 {
		logger.ErrorNoStackInfof("当前获取节点发现表协议数据格式错误")
		return false
	}
	name := string(buf[1 : nameLen+1])
	cl := CL.get(name)
	if BytesToInt(buf[lens]) == DisCoverTabs.version {
		//如果节点请求的版本号和当前最新的版本号一致，则不需要更新
		//发送原版本号获取即可
		bb := bytes.Buffer{}
		b := make([]byte, 2)
		b[0] = discAck
		b[1] = buf[lens]
		//消息头0x88
		//原版本号
		bb.Write(b)
		cl.Conn.Write(bb.Bytes())
		logger.Debug("版本号一致，无需更新")
		return true
	}
	//不一致则发送最新的节点发现表
	if cl.NodeID == name {
		bb := bytes.Buffer{}
		b := make([]byte, 1)
		b[0] = discAck
		//消息头0x88
		bb.Write(b)
		//版本号
		bb.Write(Hextob(DisCoverTabs.version)[:1])
		bb2 := bytes.Buffer{}
		for _, v := range DisCoverTabs.nodes {
			b1 := []byte(v.sid)
			b2 := []byte(v.addr)
			//下面的数据长度
			bb2.Write(Hextob(len(b1))[:1])
			bb2.Write(Hextob(len(b2))[:1])
			//数据
			bb2.Write(b1)
			bb2.Write(b2)
			bb2.Write(Hextob(v.version)[:1])
		}
		//获取表数据的长度
		lens := bb2.Len()
		bs := Hextob(lens)
		bb.Write(bs[1:])
		bb.Write(bs[:1])
		bb.Write(bb2.Bytes())
		j := 0
		//发送给节点
		for i := 0; i < 1 && j < 5; {
			_, err := cl.Conn.Write(bb.Bytes())
			if err == nil {
				i++
			}
			j++
		}
		logger.Infof("发送给%s的最新节点发现表消息成功", name)
		return true
	}
	logger.Debugf("未获取到%s的会话消息", name)
	return false
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

	var x int8
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
