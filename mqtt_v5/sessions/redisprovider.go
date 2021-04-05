package sessions

import (
	"Go-MQTT/mqtt_v5/config"
	"Go-MQTT/mqtt_v5/logger"
	"Go-MQTT/redis"
	"fmt"
	"strconv"
	"sync"
)

var _ SessionsProvider = (*redisProvider)(nil)
var (
	redisSessionsProvider = "redis"
	SSize                 int
	redisSUrl             string
	redisSPassword        string
	redisSDB              int
)

//redis池
var redisSChanPool chan redisSNode

//连接节点结构体
type redisSNode struct {
	id    string
	redis *redis.Redis
}

func redisProviderInit() {
	consts := config.ConstConf
	redisSUrl = consts.DefaultConst.Redis.RedisUrl
	redisSPassword = consts.DefaultConst.Redis.PassWord
	redisSDB = int(consts.DefaultConst.Redis.DB)
	if redisSDB > 15 {
		redisSDB = 0
	}
	SSize = int(consts.DefaultConst.Redis.RSize)
	if SSize == 0 {
		SSize = 1
	}
	if redisSessionsProvider == consts.DefaultConst.SessionsProvider {
		count := 0
		for i := 0; i < SSize; i++ {
			r := redis.Redis{}
			err := r.CreatCon("tcp", redisSUrl, redisSPassword, redisSDB)
			if err != nil {
				count++
				logger.Errorf(err, "Connect to redis-session-%d error", i)
			}
			rd := redisSNode{id: "redis-session-" + strconv.Itoa(i), redis: &r}
			redisSChanPool <- rd
		}
		if SSize > 1 {
			logger.Infof("redis Session池化处理完成，size：%d", SSize-count)
		}
		Register(redisSessionsProvider, NewRedisProvider())
	}
}

type redisProvider struct {
	st map[string]*Session
	mu sync.RWMutex
}

func NewRedisProvider() *redisProvider {
	return &redisProvider{
		st: make(map[string]*Session),
	}
}

func (this *redisProvider) New(id string) (*Session, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.st[id] = &Session{id: id}
	return this.st[id], nil
}

func (this *redisProvider) Get(id string) (*Session, error) {
	this.mu.RLock()
	defer this.mu.RUnlock()

	sess, ok := this.st[id]
	if !ok {
		return nil, fmt.Errorf("store/Get: No session found for key %s", id)
	}

	return sess, nil
}

func (this *redisProvider) Del(id string) {
	this.mu.Lock()
	defer this.mu.Unlock()
	delete(this.st, id)
}

func (this *redisProvider) Save(id string) error {
	return nil
}

func (this *redisProvider) Count() int {
	return len(this.st)
}

func (this *redisProvider) Close() error {
	this.st = make(map[string]*Session)
	return nil
}
