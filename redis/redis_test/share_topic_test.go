package redis_test

import (
	"container/list"
	"fmt"
	"strings"
	"testing"
)

func TestShare(t *testing.T) {
	fmt.Println(matchTopicS("/as/aa/aa"))
	fmt.Println(matchTopicS("/as/aa"))
	fmt.Println(matchTopicS("as/aa/"))
	fmt.Println(matchTopicS("/"))
	fmt.Println(matchTopicS(""))
}
func BenchmarkShare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matchTopicS("/asdss66666666666666666666666666666666666666dsds/aasdddsaa/aasadas/")
	}
}

// 获取需要匹配的主题
// 下面可优化的地方太多了，性能不是很好
func matchTopicS(topic string) []string {
	tp := strings.Split(topic, "/")
	ret := list.New()
	ret.PushBack(tp[0])
	// 直接限制订阅主题第一个不能是通配符,并且不能是单纯一个/,所以该方法就不做限制
	for k := range tp {
		if k == 0 {
			continue
		}
		v := tp[k]
		size := ret.Len()
		for i := 0; i < size; i++ {
			el := ret.Front()
			s := el.Value.(string)
			if s != "" && s[len(s)-1] == '#' {
				ret.MoveToBack(el)
				continue
			}
			el.Value = s + "/" + v
			ret.MoveToBack(el)
			ret.PushBack(s + "/+")
			ret.PushBack(s + "/#")
		}
	}

	da := make([]string, 0)
	for elem := ret.Front(); elem != nil; elem = elem.Next() {
		vs := elem.Value.(string)
		if vs == "" {
			continue
		}
		da = append(da, vs)
	}
	return da
}
