package colong

import (
	"Go-MQTT/mqtt_v5/config"
	"fmt"
	"github.com/go-redis/redis"
	"strconv"
)

var R *redis.Client

func init() {
	redisSUrl := config.ConstConf.DefaultConst.Redis.RedisUrl
	redisSPassword := config.ConstConf.DefaultConst.Redis.PassWord
	redisSDB := config.ConstConf.DefaultConst.Redis.DB
	if isEmpty(redisSUrl, redisSPassword) {
		panic("redis info have empty")
	}
	R = redis.NewClient(&redis.Options{DB: int(redisSDB), Password: redisSPassword, Addr: redisSUrl, Network: "tcp"})
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
    		local ks = KEYS[1].."_"..v
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
  local k = KEYS[1].."_"..KEYS[2]
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
            redis.call("del",KEYS[1].."_"..v)
		end
		return redis.call("del",KEYS[1])
	end
	return 0
`

// 获取topic的共享主题信息
func GetTopicShare(topic string) (*ShareNameInfo, error) {
	v, err := redis.NewScript(tp).Run(R, []string{topic}).Result()
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
		sni := &ShareNameInfo{V: make(map[string]map[string]int)}
		for i, da := range name {
			mp := make(map[string]int)
			for j := 0; j < len(retdata[i]); j += 2 {
				ii, err := strconv.Atoi(retdata[i][j+1].(string))
				if err != nil {
					return nil, err
				}
				mp[retdata[i][j].(string)] = ii
			}
			sni.V[da.(string)] = mp
		}
		return sni, nil
	}
	return nil, fmt.Errorf("获取数据错误")
}

// 新增一个topic下某个shareName的订阅
func SubShare(topic, shareName, nodeName string) bool {
	v, err := redis.NewScript(sr).Run(R, []string{topic, shareName, nodeName}, 1, 1).Result()
	if err != nil {
		return false
	}
	return v.(int64) > 0
}

// 取消一个topic下某个shareName的订阅
func UnSubShare(topic, shareName, nodeName string) bool {
	v, err := redis.NewScript(sr).Run(R, []string{topic, shareName, nodeName}, 0, -1).Result()
	if err != nil {
		return false
	}
	return v.(int64) > 0
}

// 删除主题
func DelTopic(topic string) error {
	_, err := redis.NewScript(del).Run(R, []string{topic}).Result()
	if err != nil {
		return fmt.Errorf("删除topic：%v下的共享信息出错：%v", topic, err)
	}
	return nil
}

type ShareNameInfo struct {
	V map[string]map[string]int
}
