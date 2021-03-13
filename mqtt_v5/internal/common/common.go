package common

import "sync"

type Type byte // 新建还是断开
const (
	CreatTy   Type = 0x00 // 有新节点
	DisConnTy Type = 0x01 // 节点掉线
)

type Node struct {
	Name    string
	Ip      string
	Version uint64
}
type SafeMap struct {
	wg *sync.RWMutex
	v  map[string]*Node
}

func (s *SafeMap) Load(k string) (n *Node, exist bool) {
	s.wg.RLock()
	defer s.wg.RUnlock()
	n, exist = s.v[k]
	return
}
func (s *SafeMap) GetMap() map[string]*Node {
	s.wg.RLock()
	defer s.wg.RUnlock()
	data := make(map[string]*Node)
	for k, v := range s.v {
		data[k] = &Node{
			Name:    v.Name,
			Ip:      v.Ip,
			Version: v.Version,
		}
	}
	return data
}
func (s *SafeMap) Store(k string, v *Node) {
	s.wg.Lock()
	defer s.wg.Unlock()
	s.v[k] = v
}
func NewSafeMap() *SafeMap {
	return &SafeMap{
		wg: &sync.RWMutex{},
		v:  make(map[string]*Node),
	}
}

// 将src完完全全拷贝到dis，并且dis之前的旧数据清除掉了
func DeepCopyMap(dis, src *SafeMap) {
	dis.wg.Lock()
	src.wg.Lock()
	defer src.wg.Unlock()
	defer dis.wg.Unlock()
	vn := make(map[string]*Node)
	for k, v := range src.v {
		vn[k] = &Node{
			Name:    v.Name,
			Ip:      v.Ip,
			Version: v.Version,
		}
	}
	dis.v = vn
}

var (
	NodeChangeFunc func(event Event) error
	NodeTable      = NewSafeMap()
	NodeTableOld   *SafeMap
	NodeChanged    = make(chan struct{}, 1)
)

type Event struct {
	Type Type
	Data map[string]string // 更新的节点 k:节点name v:节点ip
}

func CreatEvent(data map[string]string) Event {
	return Event{
		CreatTy,
		data,
	}
}
func CisConnEvent(data map[string]string) Event {
	return Event{
		DisConnTy,
		data,
	}
}
