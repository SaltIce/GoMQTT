package comment

import (
	"Go-MQTT/mqtt/internal/config"
)

var (
	ServerVersion = "3.1.0"
	//下面三行没啥用的，只能写死在service/buffer.go里面
	DefaultBufferSize     int64
	DefaultReadBlockSize  int
	DefaultWriteBlockSize int
	ClusterEnabled        bool
	DefaultKeepAlive      = 300
	DefaultConnectTimeout = 2
	DefaultAckTimeout     = 20
	DefaultTimeoutRetries = 3
	//下面两个最好一样,要是修改了要去这两个文件中（sessions/memprovider.go和topics/memtopics.go）修改对应的
	//因为不能在那里面要用这个，不然会引起相互依赖的
	//还有redis的，也在上面两个文件包下
	DefaultSessionsProvider = "mem" //保存至内存中
	DefaultTopicsProvider   = "mem"

	//下面两个修改了，也要要去auth/mock.go文件中修改
	DefaultAuthenticator        = "mockSuccess" //开启身份验证
	DefaultFailureAuthenticator = "mockFailure" //关闭身份验证，直接返回失败，拒绝所有连接
)

//从配置文件中读取配置
//要是其它地方获取这个文件中的上面那些配置信息时引起循环依赖问题，可像下面这种方式去解决
func init() {
	co := config.MyConst{}
	consts, err := config.ReadConst(&co, "mqtt/config/const.yml")
	if err != nil {
		panic(err)
	}
	ServerVersion = consts.ServerVersion
	ClusterEnabled = consts.Cluster.Enabled
	//下面三行没啥用的，只能写死在service/buffer.go里面
	DefaultBufferSize = consts.MyBuff[0].DefaultBufferSize
	DefaultReadBlockSize = consts.MyBuff[0].DefaultReadBlockSize
	DefaultWriteBlockSize = consts.MyBuff[0].DefaultWriteBlockSize

	DefaultTopicsProvider = consts.DefaultConst[0].Nodes[0].DefaultTopicsProvider
	DefaultKeepAlive = consts.DefaultConst[0].Nodes[0].DefaultKeepAlive
	DefaultConnectTimeout = consts.DefaultConst[0].Nodes[0].DefaultConnectTimeout
	DefaultAckTimeout = consts.DefaultConst[0].Nodes[0].DefaultAckTimeout
	DefaultTimeoutRetries = consts.DefaultConst[0].Nodes[0].DefaultTimeoutRetries
	//下面两个最好一样,要是修改了要去这两个文件中（sessions/memprovider.go和topics/memtopics.go）修改对应的
	//因为不能在那里面要用这个，不然会引起相互依赖的
	//还有redis的，也在上面两个文件包下
	DefaultSessionsProvider = consts.DefaultConst[0].Nodes[0].DefaultSessionsProvider //保存至内存中
	DefaultTopicsProvider = consts.DefaultConst[0].Nodes[0].DefaultTopicsProvider

	//下面两个修改了，也要要去auth/mock.go文件中修改
	DefaultAuthenticator = consts.DefaultConst[0].Nodes[0].DefaultAuthenticator               //开启身份验证
	DefaultFailureAuthenticator = consts.DefaultConst[0].Nodes[0].DefaultFailureAuthenticator //关闭身份验证，相当于不用账号密码就可以连接
}
