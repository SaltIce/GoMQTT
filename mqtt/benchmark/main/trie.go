package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

func Hextob(n int) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.LittleEndian, int16(n))
	return bytebuf.Bytes() //[10,0],暂时只有前面第一个有用
}

//字节转换成整形
func BytesToInt(b []byte) int {

	bytesBuffer := bytes.NewBuffer(b)

	var x int16
	j := 0
	for i := 0; i < 1 && j < 10; {
		err := binary.Read(bytesBuffer, binary.BigEndian, &x)
		if err == nil {
			i++
		}
		j++
	}
	return int(x)
}

//数组倒序函数
func Reverse(arr *[]byte, length int) {
	var temp byte
	for i := 0; i < length/2; i++ {
		temp = (*arr)[i]
		(*arr)[i] = (*arr)[length-1-i]
		(*arr)[length-1-i] = temp
	}
}

var pingTime = 1 * time.Minute

func main() {
	old := time.Now().Unix()
	time.Sleep(time.Minute * 1)
	fmt.Println(time.Now().Sub(time.Unix(old, 0)) > pingTime)
	fmt.Println(time.Now().Sub(time.Unix(old, 0)) < pingTime)
	//bb := Hextob(255*255)
	//fmt.Println(bb)
	//b := make([]byte,2)
	//b[0]=0
	//b[1]=104
	//fmt.Println(binary.BigEndian.Uint16(b))
	////Reverse(&bb,2)
	//fmt.Println(BytesToInt(bb))
	//go func() {
	//	topic := make([]string, 1)
	//	topic[0] = "topic/a/x/"
	//
	//	fmt.Print("Please enter your name and phonenumber: \n")
	//	//fmt.Scanln(&topic)
	//	//fmt.Println(topic)
	//	ss := colong.RouteTab{}
	//	for i, s := range topic {
	//		si := "dev" + strconv.Itoa(i)
	//		fmt.Println(ss.Insert(s, si))
	//		colong.AddTopic(s)
	//		//err := updateTrie(s)
	//		//if err!= nil{
	//		//	panic(err)
	//		//}
	//	}
	//	go colong.Nets()
	//	fmt.Println(ss.Get("main/s/w"))
	//	//print()
	//}()
	//for {
	//	time.Sleep(1 * time.Second)
	//}
}
func print() {
	fmt.Printf("len：%d ==> ", trieHead.len)
	for _, v := range trieHead.childNode {
		fmt.Printf("len：%d ，val：%s==> \n", v.len, v.val)
		print2(v)
	}
}
func print2(trie2 *trie) {
	fmt.Printf("len：%d ==> ", trie2.len)
	for _, v := range trie2.childNode {
		fmt.Printf("len：%d ，val：%s==> \n", v.len, v.val)
		print2(v)
	}
	fmt.Println()
}

//下面是生成主题树的

//头
type head struct {
	len       int
	childNode []*trie
}

//节点
type trie struct {
	childNode []*trie
	len       int
	val       string
}

//初始化一个头
var trieHead head
var rwm sync.RWMutex

func updateTrie(topic string) error {
	if topic == "" {
		return errors.New("topic 不能为''")
	}
	ok, _ := cheack(&trieHead, topic)
	fmt.Println(ok)
	return nil
}

//这里先不考虑通配符的情况
func cheack(trieHead *head, topic string) (bool, error) {
	if topic == "" {
		return false, errors.New("topic 不能为''")
	}
	a := strings.Split(topic, "/")
	//rwm.RLock()
	//defer rwm.RUnlock()
	for _, v := range trieHead.childNode {
		if v.val == a[0] {
			return deptCheack(v, a, 1), nil
		}
	}
	//没有就创建分支
	//rwm.Lock()
	//defer rwm.Unlock()
	tt := trie{}
	trieHead.childNode = append(trieHead.childNode, creatTrie(&tt, a))
	trieHead.len++
	fmt.Println("===")
	return false, nil
}

//递归遍历,存在则返回true,不存在则插入，并且返回false
func deptCheack(node *trie, topic []string, index int) bool {
	for _, v := range node.childNode {
		//topic从第二个开始，因为当前node已经和topic第一个比过了
		if v.val == topic[index] {
			index++
			return deptCheack(v, topic, index)
		}
		if v.val == topic[len(topic)-1] && v.len == 0 {
			return true
		}
	}
	tt := trie{}
	node.childNode = append(node.childNode, creatTrieIndex(&tt, topic, index))
	node.len++
	return false
}

//创建新的枝条
func creatTrie(trie2 *trie, topic []string) *trie {
	var lev = trie2
	for _, v := range topic {
		tt := new(trie)
		tt.val = v
		tt.len++
		lev.childNode = append(trie2.childNode, tt)
		lev = tt
	}
	return trie2
}

//索引后面的topic插入到枝条里面去
func creatTrieIndex(trie2 *trie, topic []string, index int) *trie {
	var lev = trie2
	for i, v := range topic {
		if i < index {
			continue
		}
		tt := new(trie)
		tt.val = v
		tt.len++
		lev.childNode = append(trie2.childNode, tt)
		lev = tt
	}
	return trie2
}
