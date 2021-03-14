package colong

import (
	"Go-MQTT/mqtt/internal/logger"
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"
)

/**
* out 的只否则将当前节点需要发送出去到其它节点的消息发出，不接收消息,除了ack消息
*
**/

//保存与其它服务端节点的通信会话
type CilentS struct {
	Sid       string   //服务端的id
	Conn      net.Conn //与服务端节点的连接
	Available bool     //是否可用
	Close     bool     //关闭标志
	rwm       sync.RWMutex
	timeS     int64
}

func getTimeSenc() int64 {
	return time.Now().Unix() //单位秒
}

//节点间的Ping的处理逻辑
func (cluster *CilentS) PingNode() bool {
	buf := make([]byte, 2)
	buf[0] = nodePingAck
	buf[1] = nodePingAck
	if _, err := cluster.Conn.Write(buf); err != nil {
		return false
	}
	return true
}

//发送接收MQ消息的第二次确认，只需发送一次即可
func (cluster *CilentS) AcknowledgementMQ02(id string) {
	logger.Debugf("添加第二次MQ消息确认：%v的等待确认", id)
	buff := bytes.Buffer{}

	buf := make([]byte, 1)
	buf[0] = mqttMsgOk2Ack
	//写入ack码
	buff.Write(buf)
	if buf, err := JsonEncode(MQTTComf{Nid: MyNetNode.Net[0].Node.In.Name, MsgID: id}); err != nil {
		return
	} else {
		buff.Write(buf)
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
						logger.Debugf("第二次MQ消息确认：%v发送成功", id)
						i++
					}
					j++
				}
			}
			if j >= 5 {
				logger.ErrorNoStackInfof("第二次MQ消息确认：%v发送失败", id)
			}
		}()
	}
}

//执行该会话复活操作
func (cluster *CilentS) Relive(conn net.Conn) {
	cluster.Available = true
	cluster.Close = false
	cluster.Conn = conn
}

//执行该会话 ping操作
func (cluster *CilentS) ping(b []byte) {
	cluster.rwm.Lock()
	defer cluster.rwm.Unlock()
	_, err := cluster.Conn.Write(b)
	if err != nil {
		logger.Info("PING 消息发送失败")
	}
}

//执行该会话死亡操作，应该在一段时间内不删除该会话，暂未去实现
func (cluster *CilentS) Death() {
	cluster.Available = false
	cluster.Close = true
	cluster.Conn.Close()
}

//发送mqtt消息
func (cluster *CilentS) OutMsg(b []byte) bool {
	cluster.rwm.Lock()
	defer cluster.rwm.Unlock()
	if cluster.Close {
		logger.ErrorNoStackInfof("当前连接已断开")
		return false
	}
	n, err := cluster.Conn.Write(b)
	if err != nil {
		logger.Error(err, "发送错误")
		return false
	}
	if n != len(b) {
		logger.Error(nil, "发送数据出现错误，导致发送字节数不对")
		return false
	}
	return true
}

type ClientList struct {
	CS  map[string]*CilentS
	rwm sync.RWMutex
}

//清除旧的、无效的连接
func (cl *ClientList) cleanOld() {
	cl.rwm.Lock()
	defer cl.rwm.Unlock()
	old := make(map[string]*CilentS)
	for k, v := range cl.CS {
		if vv := DT.get(k); vv != nil {
			if vv.sid == "" && vv.addr == "" {
				v.Death()
				old[k] = v
			}
		}
	}
	for k, _ := range old {
		delete(cl.CS, k)
	}
}
func (cl *ClientList) get(id string) *CilentS {
	return cl.CS[id]
}

//节点发现表
type discoverTab struct {
	nodes   map[string]*disNode //节点列表 sid:node
	rwm     sync.Mutex
	version int //当前发现表的版本号,只要有更新就会变化即加一，当到达127时就会变为0，主要是因为下面的字节解析用了int8，为了转换时只要一字节就行
}
type disNode struct {
	sid     string
	addr    string
	version int //当前节点的版本号,只要有更新就会变化即加一，当到达127时就会变为0，主要是因为下面的字节解析用了int8，为了转换时只要一字节就行
}

func (this *discoverTab) get(id string) *disNode {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	return this.nodes[id]
}
func (this *discoverTab) update(node map[string]*disNode) {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	//将旧表暂存至DTOld
	DTOld.update2(this.nodes)
	//更新新表
	this.nodes = node
}
func (this *discoverTab) update2(node map[string]*disNode) {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	//将旧表暂存至DTOld
	DTOld.nodes = node
}

//服务发现表
var DT discoverTab

//服务节点连接失败的保存表
var DTOver discoverTab

//服务发现表旧表，也就是上一次的表
var DTOld discoverTab

//连接到其它节点输入口的会话
var CL ClientList

//节点发现服务器会话
var NDS = CilentS{Close: false, Available: true}

//判断是否更新的通道
var tag chan bool

//判断是否更新路由表数据
var tab chan bool

func init() {
	tag = make(chan bool)
	tab = make(chan bool)
	go createCon()
	CL.CS = map[string]*CilentS{}
	DT.version = 0
	DT.nodes = make(map[string]*disNode)
	DTOld.nodes = make(map[string]*disNode)
	DTOver.nodes = make(map[string]*disNode)
	//连接节点发现服务器
	NDS.Conn = ConS()
	NDS.timeS = getTimeSenc()
	go NDSHand()
	go getTab()
}

//发送获取路由表的消息，这个只在需求启动后向节点发现服务器发送一次
func getTab() {
	for {
		select {
		case ok := <-tab:
			if !ok {
				return
			} else {
				b := make([]byte, 2)
				b[0] = getTabAck
				b[1] = getTabAck
				if _, err := NDS.Conn.Write(b); err != nil {
					tab <- false
				}
			}
		}
	}
}

var pp = true

//当需要更新节点列表连接时
//处理连接其它节点的逻辑
func createCon() {
	for {
		select {
		case c := <-tag:
			if c {
				if pp {
					tab <- true
					pp = false
				}
				//用来保存连接失败的节点
				dels := make(map[string]string)
				//如果有连接失败的节点，这里重新连接
				if len(DTOver.nodes) > 0 {
					for _, v := range DTOver.nodes {
						cs := CilentS{Available: true, Close: false}
						if c := CreateS(v.sid, v.addr); c != nil {
							cs.Conn = c
							cs.Sid = v.sid
							go handleInMQ(&cs)
							CL.CS[v.sid] = &cs
						} else {
							dels[v.sid] = v.addr //负责收集没有连接成功的节点
						}
					}
				} else {
					//遍历新的节点发现表
					for k, v := range DT.nodes {
						if k == MyNetNode.Net[0].Node.In.Name {
							//自己就不要连自己了
							continue
						}
						//从连接会话集合中寻找是否存在对应的会话，存在则更新（通过发送一次心跳测试连接是否可用），不存在则创建
						if vv := CL.get(v.sid); vv != nil && vv.Available {
							//连接不可用，需更新
							if !vv.PingNode() {
								cs := CilentS{Available: true, Close: false}
								if c := CreateS(v.sid, v.addr); c != nil {
									cs.Conn = c
									cs.Sid = v.sid
									go handleInMQ(&cs)
									CL.CS[v.sid] = &cs
								} else {
									dels[v.sid] = v.addr //负责收集没有连接成功的节点
								}
							}
							////如果存在之前的会话
							////判断一次版本号和上一次的是否一致,一致代表不用更新
							//old := DTOld.get(vv.Sid)
							////如果旧表中有此数据则进入
							//if old != nil {
							//	//判断版本号是否一致
							//	if DTOld.get(vv.Sid).version == DT.get(vv.Sid).version {
							//		continue
							//	} else { //不一致就重新连接
							//		cs := CilentS{Available: true, Close: false}
							//		if c := CreateS(v.sid, v.addr); c != nil {
							//			cs.Conn = c
							//			cs.Sid = v.sid
							//			go handleInMQ(&cs)
							//			CL.CS[v.sid] = &cs
							//		} else {
							//			dels[v.sid] = v.addr //负责收集没有连接成功的节点
							//		}
							//	}
							//}
						} else {
							cs := CilentS{Available: true, Close: false}
							if c := CreateS(v.sid, v.addr); c != nil {
								cs.Conn = c
								cs.Sid = v.sid
								go handleInMQ(&cs)
								CL.CS[v.sid] = &cs
							} else {
								dels[v.sid] = v.addr //负责收集没有连接成功的节点
							}
						}
					}
					CL.cleanOld() //清除旧的、无效的连接
				}
				if len(dels) > 0 {
					//处理未连接成功的节点
					s := bytes.Buffer{}
					s.Write([]byte("下列节点连接连接失败\n"))
					for k, v := range dels {
						s.Write([]byte(k))
						s.Write([]byte("--"))
						s.Write([]byte(v))
						s.Write([]byte("\n"))
					}
					//重新再连接一遍
					tag <- true
					logger.ErrorNoStackInfof(s.String())
				}
			}
		}
	}
}

//每个与其它节点的ack确认数据的处理方法
func handleInMQ(cs *CilentS) {
	b := make([]byte, 4096)
	for {
		n, err := cs.Conn.Read(b)
		if err != nil {
			logger.ErrorNoStackInfof("连接断开%v", cs.Sid)
			cs.Death()
			return
		}
		switch b[0] {
		case mqttMsgOkAck:
			mqttMsgOkHandle(b[1:n], cs)

		}
	}
}

var pingTime = 2 * time.Second //超时时间 一分钟
//处理与节点发现服务器的数据交互
func NDSHand() {
	// 在 主go程中， 获取服务器回发数据。
	buf2 := make([]byte, 4096)
	go func() {
		for {
			if time.Now().Sub(time.Unix(NDS.timeS, 0)) > pingTime {
				logger.Debug("发送PING消息")
				NDS.ping(getPingMsg())
			}
			time.Sleep(time.Second * 30)
		}
	}()
	for {
		// 借助 socket 从服务器读取 数据。
		n, err := NDS.Conn.Read(buf2)
		if n == 0 {
			logger.Info("客户端检查到节点发现服务器，关闭连接， 本端也退出")
			return
		}
		if err != nil {
			logger.Error(err, "节点发现服务器 os.Stdin.Read err:")
			return
		}
		handle(buf2[:n])
	}
}

//组装一个ping消息
func getPingMsg() []byte {
	bb := make([]byte, 2)
	bb[0] = pingAck
	bb[1] = pingAck
	return bb
}

//处理节点发现服务器发来的节点发现表数据
func handle(buf []byte) {
	switch buf[0] {
	case infoAck: // 0x89 + 版本号
		//version:=BytesToInt(buf[1])
		//发送请求节点发现表消息
		getDiscTab()
		//更新时间戳
		NDS.timeS = getTimeSenc()
	case discAck: // 0x88 + 版本号（1bit）+ 数据长度（2bit）+表数据（[id长度+地址长度+id+地址][id长度+地址长度+id+地址]...）
		parse(buf)
		//更新时间戳
		NDS.timeS = getTimeSenc()
	case pingAck:
		pingHandle(NDS, buf)
		//更新时间戳
		NDS.timeS = getTimeSenc()
	case getTabAck:
		tabPushC(buf[1:])

	default:
		logger.ErrorNoStackInfof("接收到节点发现服务器异常数据：%v", buf)
		//无法解析的数据
	}
}

//负责处理接收到第一次mqtt消息确认的处理，要删除MQTT消息确认表中数据
func mqttMsgOkHandle(buf []byte, cs *CilentS) bool {
	mq := MQTTComf{}
	if err := JsonDecode(buf, &mq); err != nil {
		return false
	} else {
		//删除等待确认
		msgAwaitAck.del(mq.MsgID, mq.Nid)
		//发送第二次mqtt消息确认
		cs.AcknowledgementMQ02(mq.MsgID)
		return true
	}
}

//解析路由表
func tabPushC(buf []byte) {
	logger.Debugf("开始解析路由表数据：%v", buf)
	lens := int(binary.LittleEndian.Uint16(buf[:2]))
	if lens != len(buf[2:]) {
		logger.ErrorNoStackInfof("路由表数据解析失败，字节数组长度错误")
		return
	}
	if !cluster.Decode(buf[2:]) {
		logger.ErrorNoStackInfof("路由表数据解析失败")
		return
	}
	logger.Debugf("解析路由表数据成功：%v", cluster.route.cm)
}

//处理ping 心跳消息
func pingHandle(clusterNode CilentS, pingMsg []byte) bool {
	//简单的判断，这个的ping消息：两个pingAck
	if len(pingMsg) != 2 {
		return false
	}
	if pingMsg[0] == pingMsg[1] {
		return true
	}
	return false
}

//发送请求节点发现表消息
// 0x88 获取节点发现表协议： 消息标志位（1bit）+ 数据域长度（客户端节点名称长度（1bit）+1）+ 数据域（名称 + 版本号（1bit） ）// 暂时不加校验码
func getDiscTab() {
	bb := bytes.Buffer{}
	b1 := make([]byte, 1)
	b1[0] = discAck
	bb.Write(b1)
	//数据域长度=节点id长度+1字节（版本号的）
	bb.Write(Hextob(len(MyNetNode.Net[0].Node.In.Name) + 1)[:1])
	//节点id
	bb.Write([]byte(MyNetNode.Net[0].Node.In.Name))
	//版本号
	bb.Write(Hextob(DT.version)[:1])
	j := 0
	for i := 0; i < 1 && j < 5; {
		_, err := NDS.Conn.Write(bb.Bytes())
		if err == nil {
			i++
		}
		j++
	}
	logger.Debugf("发送请求节点发现表消息：%v", bb.Bytes())
}

//解析出节点发现表数据
// 0x88 + 版本号(1bit) + 数据域长度 + 节点数据
func parse(buf []byte) {
	//解析节点发现表
	logger.Debugf("开始解析节点发现表：%v", buf)
	//版本号
	version := BytesToInt(buf[1])
	//数据长度
	lens := int(binary.BigEndian.Uint16(buf[2:4]))
	dataLen := len(buf[4:])
	if lens != dataLen || dataLen <= 0 {
		//判断数据长度和表数据长度是否一致
		logger.Debug("节点发现表消息的数据长度和表数据长度不一致")
		tag <- false
		return
	}
	DT.version = version
	idTag := 4
	addrTag := 5
	end := 4
	node := make(map[string]*disNode)
	for i := 0; i < 1; {
		idLen := int(buf[idTag])
		addrLen := int(buf[addrTag])
		id := string(buf[idTag+2 : idTag+2+idLen])
		addr := string(buf[addrTag+1+idLen : addrTag+1+idLen+addrLen])
		nodeVersion := BytesToInt(buf[addrTag+1+idLen+addrLen])
		node[id] = &disNode{sid: id, addr: addr, version: nodeVersion}
		end = end + idLen + addrLen + 3
		idTag = end
		addrTag = end + 1
		if end == lens+4 {
			i++
		}
	}
	DT.update(node)
	logger.Infof("更新节点发现表：%v", node)
	tag <- true
}

//连接节点发现服务器
// 0x00 连接消息协议： 标志位（1bit）+ 数据域长度（客户端节点名称长度（1bit）+端口长度（1bit））+ 数据域（名称+port）
func ConS() net.Conn {
	if MyNetNode.Discover.Addr == "" {
		panic("没有节点发现服务器地址")
	}
	conn, err := net.Dial("tcp", MyNetNode.Discover.Addr)
	if err != nil {
		logger.Errorf(err, "-- Dial discover server err：%s", MyNetNode.Discover.Addr)
		panic(err)
		return nil
	}
	//发送连接信息
	buf := bytes.Buffer{}
	b := make([]byte, 1)
	//消息头
	b[0] = ackCon
	buf.Write(b)
	//发送节点输入口的id与port
	dev := MyNetNode.Net[0].Node.In.Name
	ports := MyNetNode.Net[0].Node.In.Port
	if dev == "" || ports == "" {
		panic("未获取到节点输入口的id与port")
	}
	buf.Write(Hextob(len(dev))[:1])
	buf.Write(Hextob(len(ports))[:1])
	buf.Write([]byte(dev))
	buf.Write([]byte(ports))
	j := 0
	for i := 0; i < 1 && j < 5; {
		if _, err := conn.Write(buf.Bytes()); err == nil {
			i++
		}
		j++
	}
	return conn
}

//连接其它节点
func CreateS(name, addr string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		logger.Errorf(err, "%s -- Dial err：%s", name, addr)
		return nil
	}
	//发送连接信息
	buf := bytes.Buffer{}
	b := make([]byte, 1)
	b[0] = ackCon
	buf.Write(b)
	dev := MyNetNode.Net[0].Node.Out.Name
	buf.Write(Hextob(len(dev))[:1])
	buf.Write([]byte(dev))
	j := 0
	for i := 0; i < 1 && j < 5; {
		if _, err := conn.Write(buf.Bytes()); err == nil {
			i++
		}
		j++
	}
	logger.Infof("连接节点：%v，addr：%v成功", name, addr)
	return conn
}

//推送主题数据至节点发现服务器
func pushTopicToDisc(b []byte) {
	j := 0
	for i := 0; i < 1 && j < 5; {
		_, err := NDS.Conn.Write(b)
		if err == nil {
			i++
		}
		j++
	}
	logger.Debugf("发送主题数据至节点发现服务器：%v", b)
}

//通知其它节点更新topic，tag为0x00表示添加，0x01表示删除
func outTopic(topic string, tag byte) {
	buf := getTopicBytes(topic, tag)
	//待确认集合
	tt := make([]string, len(CL.CS)+1)

	//发送主题数据至节点发现服务器
	pushTopicToDisc(buf)
	tt[0] = "nodeDiscoverServer"
	k := 1
	j := 0
	for _, v := range CL.CS {
		if v.Close {
			logger.Debugf("节点%v，已经关闭无法发送数据", v.Sid)
			continue
		}
		for i := 0; i < 1 && j < 5; {
			//发送出去
			if _, err := v.Conn.Write(buf); err == nil {
				i++
				break
			}
			j++
		}
		//保存至待确认集合中去
		tt[k] = v.Sid
		k++
	}
	//将待确认集合同主题存入awaitAck
	awaitAck.add(topic, tt)
}

//组合topic消息
func getTopicBytes(topic string, tag byte) []byte {
	buf := bytes.Buffer{}
	b := make([]byte, 1)
	b[0] = ackPush
	//表示添加还是删除
	bb := make([]byte, 1)
	bb[0] = tag
	buf.Write(b)
	//当前节点的id
	b1 := []byte(MyNetNode.Net[0].Node.Out.Name)
	//主题
	b2 := []byte(topic)
	//写id长度
	buf.Write(Hextob(len(b1))[:1])
	//写topic长度
	buf.Write(Hextob(len(b2))[:1])
	//写id
	buf.Write(b1)
	//写topic
	buf.Write(b2)
	//写标志，增加还是减少
	buf.Write(bb)
	return buf.Bytes()
}
func AddTopic(topic string) {
	outTopic(topic, addAck)
}
func DelTopic(topic string) {
	outTopic(topic, delAck)
}
