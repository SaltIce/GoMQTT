package nodediscover

import (
	"Go-MQTT/Logger"
	"Go-MQTT/mqtt_v5/config"
	"Go-MQTT/mqtt_v5/internal/common"
	"Go-MQTT/mqtt_v5/logger"
	"encoding/binary"
	"fmt"
	"github.com/samuel/go-zookeeper/zk"
	uuid "github.com/satori/go.uuid"
	"math"
	"net"
	"strings"
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
var (
	zkCoon   *zk.Conn
	nodeName string // 当前节点的唯一name
	enable   bool
	version  uint64 = 0 // 版本号
	hostIp   string
	timeOut  = 5
	mqRoot   = "/gomq"
)

func init() {
	enable = config.ConstConf.Cluster.Enabled
	if !enable {
		return
	}
	var url = make([]string, 0)
	nodeName = strings.TrimSpace(config.ConstConf.Cluster.Name)
	if nodeName == "" {
		nodeName = uuid.NewV4().String()
	}
	hostIp = strings.TrimSpace(config.ConstConf.Cluster.HostIp)
	if hostIp == "" {
		panic("host ip is empty")
	}
	if config.ConstConf.Cluster.ZkTimeOut > 0 && config.ConstConf.Cluster.ZkTimeOut < 60 {
		timeOut = config.ConstConf.Cluster.ZkTimeOut
	}
	if strings.TrimSpace(config.ConstConf.Cluster.ZkMqRoot) != "" {
		mqRoot = strings.TrimSpace(config.ConstConf.Cluster.ZkMqRoot)
	}
	url2 := config.ConstConf.Cluster.ZkUrl
	if url2 != "" {
		url3 := strings.Split(url2, ",")
		for _, v := range url3 {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			url = append(url, v)
		}
	}
	if len(url) == 0 {
		url = append(url, "127.0.0.1:2181")
	}
	c, err := GetConnect(url)
	checkError(err)
	zkCoon = c
	checkError(RegistServer([]byte(nodeName), []byte(hostIp)))
	update()
	AddWatch(mqRoot)
}
func GetConnect(zkUrl []string) (conn *zk.Conn, err error) {
	conn, _, err = zk.Connect(zkUrl, time.Duration(timeOut)*time.Second)
	if err != nil {
		fmt.Println(err)
	}
	return
}

// 注册服务 nodeName 为节点名称，hostIp为节点ip
func RegistServer(nodeName []byte, hostIp []byte) (err error) {
	if exist, _, err := zkCoon.Exists(mqRoot); err == nil && !exist {
		_, _ = zkCoon.Create(mqRoot, nil, 0, zk.WorldACL(zk.PermAll))
	}
	nodeNameLen := len(nodeName)
	hostIpLen := len(hostIp)
	buf := make([]byte, 8)
	data := make([]byte, 0)
	n := binary.PutUvarint(buf, uint64(nodeNameLen)) // name
	data = append(data, buf[:n]...)
	data = append(data, nodeName...)
	n = binary.PutUvarint(buf, uint64(hostIpLen)) //ip
	data = append(data, buf[:n]...)
	data = append(data, hostIp...)
	n = binary.PutUvarint(buf, version) // 版本号
	data = append(data, buf[:n]...)
	version++
	if version >= math.MaxUint64 {
		version = 0
	}
	_, err = zkCoon.Create(mqRoot+"/"+string(nodeName), data, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	return
}

// 获取节点列表
func GetServerList() (list []string, err error) {
	list, _, err = zkCoon.Children(mqRoot)
	return
}

// 获取节点name的数据
func GetServerData(nodeName string) (common.Node, error) {
	data, _, err := zkCoon.Get(mqRoot + "/" + nodeName)
	if err != nil {
		return common.Node{}, err
	}
	nameLen, n := binary.Uvarint(data)
	name := make([]byte, nameLen)
	copy(name, data[n:n+int(nameLen)])
	ipLen, n2 := binary.Uvarint(data[n+int(nameLen):])
	ip := make([]byte, ipLen)
	pre := n + n2 + int(nameLen)
	copy(ip, data[pre:pre+int(ipLen)])
	version, _ := binary.Uvarint(data[n+int(nameLen):])
	node := common.Node{
		Name:    string(name),
		Ip:      string(ip),
		Version: version,
	}
	return node, nil
}
func AddWatch(root string) {
	_, s, e, err := zkCoon.ChildrenW(mqRoot)
	checkError(err)
	logger.Infof("zk state : %+v", s)
	go func() {
		for {
			event := <-e
			_, s, e, err = zkCoon.ChildrenW(root)
			checkError(err)
			logger.Infof("zk event.Path : %+v", event.Path)
			logger.Infof("zk event.Type : %+v", event.Type)
			logger.Infof("zk state : %+v", s)
			switch event.Type {
			case zk.EventNodeChildrenChanged:
				// 更新连接
				update()
			}
		}
	}()
}
func update() {
	// 更新连接
	list, err := GetServerList()
	if err != nil {
		Logger.PError(err, "节点监听获取list失败")
		return
	}
	for _, v := range list {
		if v == nodeName {
			continue
		}
		node, err := GetServerData(v)
		if err != nil {
			Logger.PError(err, "节点监听获取%v的数据失败", v)
			continue
		}
		ip := node.Ip
		if old, ok := common.NodeTable.Load(v); ok {
			if old.Ip == ip && node.Version == old.Version {
				continue
			}
			old.Ip = ip
			old.Version = node.Version
			if old.Version >= math.MaxUint64 {
				old.Version = 0
			}
		} else {
			common.NodeTable.Store(node.Name, &node)
		}
	}
	common.NodeChanged <- struct{}{}
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

/**
 * 测试临时节点
 * @return {[type]}
 */
func test2() {
	conn, err := GetConnect([]string{"10.112.26.131:2181"})
	checkError(err)
	defer conn.Close()
	fmt.Println(conn.Create("/mq", []byte("1232"), 0, zk.WorldACL(zk.PermAll)))
	fmt.Println(conn.Get("/mq"))
	_, s, e, err := conn.ChildrenW("/mq")
	checkError(err)
	fmt.Printf("%+v\n", s)
	go func() {
		for {
			event := <-e
			fmt.Println(event.Path)
			fmt.Println(event.State)
			fmt.Println(event.Type)
			_, s, e, err = conn.ChildrenW("/mq")
			checkError(err)
		}
	}()
	fmt.Println(conn.Create("/mq/go_servers2", []byte("1232"), 0, zk.WorldACL(zk.PermAll)))
	fmt.Println(conn.Create("/mq/go_servers2/1", []byte("222"), zk.FlagEphemeral, zk.WorldACL(zk.PermAll)))
	ch, s, err := conn.Children("/mq")
	checkError(err)
	fmt.Println(ch, s)
	time.Sleep(2 * time.Second)
	ch, s, err = conn.Children("/mq/go_servers2")
	checkError(err)
	fmt.Println(ch, s)
	for _, c := range ch {
		conn.Delete("/mq/go_servers2/"+c, 0)
	}
	ch, s, err = conn.Children("/mq/go_servers2")
	checkError(err)
	fmt.Println(ch)
	time.Sleep(2 * time.Second)
	conn.Delete("/mq/go_servers2", 0)
	time.Sleep(2 * time.Second)

}
func handleCient(conn net.Conn, port string) {
	defer conn.Close()

	daytime := time.Now().String()
	conn.Write([]byte(port + ": " + daytime))
}
