package redis

import (
	"Go-MQTT/mqtt_v5/config"
	"fmt"
	"github.com/go-redis/redis"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

var (
	r            *redis.Client
	cacheGlobal  *cacheInfo
	cacheOutTime = 1 * time.Second
)

type tn struct {
	v *ShareNameInfo
	*time.Timer
}
type global map[string]*tn
type cacheInfo struct {
	sync.RWMutex
	global
}

func init() {
	redisSUrl := config.ConstConf.DefaultConst.Redis.RedisUrl
	redisSPassword := config.ConstConf.DefaultConst.Redis.PassWord
	redisSDB := config.ConstConf.DefaultConst.Redis.DB
	if isEmpty(redisSUrl) {
		panic("redis info have empty")
	}
	r = redis.NewClient(&redis.Options{DB: int(redisSDB), Password: redisSPassword, Addr: redisSUrl, Network: "tcp"})
	_, err := r.Ping().Result()
	if err != nil {
		panic(err)
	}
	cacheGlobal = &cacheInfo{
		RWMutex: sync.RWMutex{},
		global:  make(map[string]*tn),
	}
	cacheGlobal.autoClea()
}

// 定时清理缓存
func (cg *cacheInfo) autoClea() {
	go func() {
		c := time.Tick(cacheOutTime)
		for _ = range c {
			cacheGlobal.Lock()
			for k, v := range cacheGlobal.global {
				select {
				case <-v.C:
					delete(cacheGlobal.global, k)
				default:
					continue
				}
			}
			cacheGlobal.Unlock()
		}
	}()
}
func isEmpty(str ...string) bool {
	for _, v := range str {
		if v == "" {
			return true
		}
	}
	return false
}

// 获取数据脚本
var tp = `
--[ 获取sharename成员 --]
	local shareName = redis.call("smembers",KEYS[1])
	local ret = {}
	local cc = {}
	local i = 1
	if shareName ~= nil
	then
--[ 遍历sharename，拼接KEYS[1] 获取该共享组下的信息--]
		for k, v in ipairs(shareName) do
			local aa = {}
			cc[k] = v
    		local ks = KEYS[1].."/"..v
            local c = redis.call("hgetall",ks)
			if c ~= nil
			then
				for k2, v2 in ipairs(c) do
					aa[k2] = v2
				end
			end
			ret[k] = aa
			i = i +1
		end
	end
	ret[i] = cc
	return ret
`

// 更新脚本
var sr = `
  redis.call("sadd",KEYS[1],KEYS[2])
  local k = KEYS[1].."/"..KEYS[2]
  local value = redis.call("hget",k,KEYS[3])
  if value == nil
   then
    return redis.call("hset",k,KEYS[3],ARGV[1])
  else 
    local p = tonumber(value)
    if p == nil
    then 
      return redis.call("hset",k,KEYS[3],ARGV[1])
    else
      if p < 0
      then return redis.call("hincrby",k,KEYS[3],-1 * p)
      else 
        if p + ARGV[2] < 0
        then return redis.call("hincrby",k,KEYS[3],0)
        else return redis.call("hincrby",k,KEYS[3],ARGV[2])
        end
      end
    end
  end
`

// 删除脚本
var del = `
	local shareName = redis.call("smembers",KEYS[1])
	if shareName ~= nil
	then
		for k, v in ipairs(shareName) do
            redis.call("del",KEYS[1].."/"..v)
		end
		return redis.call("del",KEYS[1])
	end
	return 0
`
var merge = &Group{}

func reqMerge(topic string) (*ShareNameInfo, error) {
	ret := merge.DoChan(topic, func() (interface{}, error) {
		v, err := redis.NewScript(tp).Run(r, []string{topic}).Result()
		if err != nil {
			return nil, fmt.Errorf("脚本执行错误：%v", err)
		}
		if v1, ok := v.([]interface{}); ok {
			ln := len(v1)
			if ln == 0 || ln == 1 {
				return nil, fmt.Errorf("未查询到数据")
			}
			name := v1[ln-1].([]interface{})
			retdata := make([][]interface{}, 0)
			for i := 0; i < ln-1; i++ {
				retdata = append(retdata, v1[i].([]interface{}))
			}
			sni := NewShareNameInfo()
			for i, da := range name {
				tv := 0
				mp := make(map[string]int)
				for j := 0; j < len(retdata[i]); j += 2 {
					ii, err := strconv.Atoi(retdata[i][j+1].(string))
					if err != nil {
						return nil, err
					}
					mp[retdata[i][j].(string)] = ii
					tv += ii
				}
				sni.V[da.(string)] = mp
				sni.t[da.(string)] = tv
			}
			cacheGlobal.Lock()
			cacheGlobal.global[topic] = &tn{
				v:     sni,
				Timer: time.NewTimer(cacheOutTime),
			}
			cacheGlobal.Unlock()
			return sni, nil
		}
		return nil, fmt.Errorf("获取数据错误")
	})
	select {
	case p := <-ret:
		if p.Err != nil {
			return &ShareNameInfo{}, p.Err
		}
		return p.Val.(*ShareNameInfo), nil
	case <-time.After(time.Millisecond * 300):
		return nil, fmt.Errorf("请求超时")
	}
}

// 获取topic的共享主题信息
func GetTopicShare(topic string) (*ShareNameInfo, error) {
	cacheGlobal.RLock()
	if v, ok := cacheGlobal.global[topic]; ok {
		defer cacheGlobal.RUnlock()
		return v.v, nil
	}
	cacheGlobal.RUnlock()
	return reqMerge(topic)
}

// 新增一个topic下某个shareName的订阅
// A：$share/a_b/c
// B：$share/a/b_c
// 如果采用非/,+,#的拼接符
// 会出现redis中冲突的情况，
// 可以考虑换一个拼接符 '/'，因为$share/{shareName}/{filter} 中shareName中不能出现'/'的
// 上述已修改为 '/'
func SubShare(topic, shareName, nodeName string) bool {
	v, err := redis.NewScript(sr).Run(r, []string{topic, shareName, nodeName}, 1, 1).Result()
	if err != nil {
		return false
	}
	return v.(int64) > 0
}

// 取消一个topic下某个shareName的订阅
func UnSubShare(topic, shareName, nodeName string) bool {
	v, err := redis.NewScript(sr).Run(r, []string{topic, shareName, nodeName}, 0, -1).Result()
	if err != nil {
		return false
	}
	return v.(int64) > 0
}

// 删除主题
func DelTopic(topic string) error {
	_, err := redis.NewScript(del).Run(r, []string{topic}).Result()
	if err != nil {
		return fmt.Errorf("删除topic：%v下的共享信息出错：%v", topic, err)
	}
	return nil
}

type ShareNameInfo struct {
	sync.RWMutex
	V map[string]map[string]int
	t map[string]int // 每个shareName下的总数
}

func NewShareNameInfo() *ShareNameInfo {
	return &ShareNameInfo{
		V: make(map[string]map[string]int),
		t: make(map[string]int),
	}
}

// 返回不同共享名称组应该发送给哪个节点的数据
// 返回节点名称：节点需要发送的共享组名称
func (s *ShareNameInfo) RandShare() map[string][]string {
	s.RLock()
	defer s.RUnlock()
	ret := make(map[string][]string)
	for shareName, nodes := range s.V {
		if s.t[shareName] <= 0 {
			continue
		}
		randN := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(s.t[shareName])
		for k, v := range nodes {
			randN -= v
			if randN > 0 {
				continue
			}
			ret[k] = append(ret[k], shareName)
		}
	}
	return ret
}
