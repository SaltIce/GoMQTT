package config

//配置文件中字母要小写，结构体属性首字母要大写
type Myconf struct {
	Ipport    string
	StartSendTime string
	SendMaxCountPerDay int
	Devices []Device
	WarnFrequency int
	SendFrequency int
	Const []DefaultConsts

}
//配置
type MyConst struct {
	DefaultConst []DefaultConsts
	MyConst []myConsts
	ServerVersion string
	MyBuff []buff
	ServerConf config
	MyAuth []MyAuth
	Cluster cluster
}
type cluster struct {
	Enabled bool
}
type MyAuthCofig struct {
	MyAuth []MyAuth
}
type Device struct {
	DevId string
	Nodes []Node
}

type Node struct {
	PkId string
	BkId string
	Index string
	MinValue float32
	MaxValue float32
	DataType string
}
type DefaultConsts struct {
	DevId string
	Nodes []con
}
type MyAuth struct {
	Auth []auth
}
type Version struct {
	ServerVersion string
}
type myConsts struct {
	DevId string
	Nodes []mycon
}
type con struct {
	DefaultKeepAlive int
	DefaultConnectTimeout int
	DefaultAckTimeout int
	DefaultTimeoutRetries int
	DefaultSessionsProvider string
	DefaultTopicsProvider string
	DefaultAuthenticator string
	DefaultFailureAuthenticator string
}
type mycon struct {
	Redis []redis
	Mysql []mysql
	RedisSessionsProvider string
	RedisTopicsProvider string
	RedisAuthenticator string
	RedisFailureAuthenticator string
}
type redis struct {
	RedisUrl string
	PassWord string
	RSize int
	DB int
}
type mysql struct {
	MysqlUrl string
	Account string
	PassWord string
	DataBase string
	MysqlPoolSize int
}
type auth struct {
	Open bool
	DefaultName string
	DefaultPwd string
}
type buff struct {
	DefaultBufferSize     int64
	DefaultReadBlockSize  int
	DefaultWriteBlockSize  int
}
type config struct {
	SessionsProvider string
	TopicsProvider string
	Authenticator string
}

