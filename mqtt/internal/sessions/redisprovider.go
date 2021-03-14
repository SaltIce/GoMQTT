// Copyright (c) 2014 The SurgeMQ Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sessions

import (
	"Go-MQTT/mqtt/internal/config"
	"Go-MQTT/mqtt/internal/logger"
	"Go-MQTT/redis"
	"fmt"
	"strconv"
	"sync"
)

var _ SessionsProvider = (*redisProvider)(nil)
var (
	redisSessionsProvider   = ""
	serverSessionsProvider2 = " "
	redisSOpen              bool
	SSize                   int
	redisSUrl               string
	redisSPassword          string
	redisSDB                int
)

//redis池
var redisSChanPool chan redisSNode

//连接节点结构体
type redisSNode struct {
	id    string
	redis *redis.Redis
}

func init() {
	co := config.MyConst{}
	consts, err := config.ReadConst(&co, "mqtt/config/const.yml")
	if err != nil {
		panic(err)
	}
	redisSessionsProvider = consts.MyConst[0].Nodes[0].RedisSessionsProvider
	serverSessionsProvider2 = consts.ServerConf.SessionsProvider
	redisSUrl = consts.MyConst[1].Nodes[0].Redis[0].RedisUrl
	redisSPassword = consts.MyConst[1].Nodes[0].Redis[0].PassWord
	redisSDB = consts.MyConst[1].Nodes[0].Redis[0].DB
	SSize = consts.MyConst[1].Nodes[0].Redis[0].RSize
	if serverSessionsProvider2 == redisSessionsProvider {
		redisSOpen = true
	}
}
func init() {
	if redisSessionsProvider != "" {
		if redisSOpen {
			Register(redisSessionsProvider, NewRedisProvider())
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
			logger.Infof("redis Session池化处理完成，size：%d", SSize-count)
		}
	} else {
		fmt.Println("获取默认RedisSessionsProvider失败")
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
