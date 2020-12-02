package auth

import (
	"Go-MQTT/mqtt/config"
	"Go-MQTT/mqtt/logger"
	"strconv"
	"Go-MQTT/redis"
)

type redisAuthenticator bool

const (
	redisMockSuccessAuthenticator redisAuthenticator = true
	redisMockFailureAuthenticator redisAuthenticator = false
)

var (
	RedisAuthenticator        string
	RedisFailureAuthenticator string
	isInit                    bool
	size                      int
	redisUrl                  string
	redisPassword             string
	redisDB                   int
)

//redis池
var redisChanPool chan redisNode

//连接节点结构体
type redisNode struct {
	id    string
	redis *redis.Redis
}

func init() {
	co := config.MyConst{}
	consts, err := config.ReadConst(&co, "mqtt/config/const.yml")
	if err != nil {
		panic(err)
	}
	if consts.MyConst[0].Nodes[0].Redis[0].RSize <= 0 {
		size = 10 //要是没有设置，默认10个
	} else {
		size = consts.MyConst[0].Nodes[0].Redis[0].RSize
	}
	redisChanPool = make(chan redisNode, size)
	redisUrl = consts.MyConst[0].Nodes[0].Redis[0].RedisUrl
	redisPassword = consts.MyConst[0].Nodes[0].Redis[0].PassWord
	redisDB = consts.MyConst[0].Nodes[0].Redis[0].DB
	RedisAuthenticator = consts.MyConst[0].Nodes[0].RedisAuthenticator
	RedisFailureAuthenticator = consts.MyConst[0].Nodes[0].RedisFailureAuthenticator
	//判断是否采用redis,采用才初始化redis池
	if consts.ServerConf.Authenticator == consts.MyConst[0].Nodes[0].RedisAuthenticator {
		isInit = true
	} else {
		isInit = false
	}
}

func init() {
	if isInit {
		logger.Info("开启redis进行账号认证")
		count := 0
		for i := 0; i < size; i++ {
			r := redis.Redis{}
			err := r.CreatCon("tcp", redisUrl, redisPassword, redisDB)
			if err != nil {
				count++
				logger.Errorf(err, "Connect to redis-%d error", i)
			}
			rd := redisNode{id: "redis-" + strconv.Itoa(i), redis: &r}
			redisChanPool <- rd
		}
		logger.Infof("redis池化处理完成，size：%d", size-count)
		if openTestAuth {
			rd := <-redisChanPool
			defer func() { redisChanPool <- rd }()
			r := rd.redis
			//插入默认账号密码
			r.SetNX(defaultName, defaultPwd)
		}
		logger.Info("开启redis账号认证")
	} else {
		logger.Info("未开启redis账号认证")
	}
}
func init() {
	if isInit {
		Register(RedisAuthenticator, redisMockSuccessAuthenticator)        //开启验证
		Register(RedisFailureAuthenticator, redisMockFailureAuthenticator) //关闭认证，直接失败，拒绝所有连接
	}
}
func (this redisAuthenticator) Authenticate(id string, cred interface{}) error {
	if this {
		if cid, ok := redisCheck(id, cred); ok {
			logger.Infof("redis : 账号：%v，密码：%v，登陆-成功，clientID==%s", id, cred, cid)
			return nil
		}
		logger.Infof("redis : 账号：%v，密码：%v，登陆-失败", id, cred)
		return ErrAuthFailure
	}
	logger.Info("当前未开启账号验证，取消一切连接。。。")
	return ErrAuthFailure
}
func redisCheck(id string, cred interface{}) (string, bool) {
	rd := <-redisChanPool
	defer func() { redisChanPool <- rd }()
	r := rd.redis
	b, err := r.GetV(id)
	if err != nil {
		logger.Errorf(err, "redis get %s failed:", id)
		return "", false
	}
	if string(b) == cred.(string) {
		return "id", true
	} else {
		return "", false
	}
}
