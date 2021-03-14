// mqtt 压测
package test

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	//导入mqtt包
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var f MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Printf("TOPIC: %s\n", msg.Topic())
	fmt.Printf("MSG: %s\n", msg.Payload())
}
var fail_nums int = 0

func TestGoMqtt(t *testing.T) {
	var n int64 = 0
	go createSubTask(&n)
	//生成连接的客户端数
	c := flag.Uint64("n", 300, "client nums")
	flag.Parse()
	nums := int(*c)
	wg := sync.WaitGroup{}
	for i := 0; i < nums; i++ {
		wg.Add(1)
		time.Sleep(5 * time.Millisecond)
		go createTask(i, &wg)
	}
	wg.Wait()
	time.Sleep(3 * time.Second)
	fmt.Println("收到消息数：", n)
}
func createSubTask(n *int64) {
	opts := MQTT.NewClientOptions().AddBroker("tcp://127.0.0.1:1887").SetUsername("admin").SetPassword("123456")
	opts.SetClientID(fmt.Sprintf("go-simple-client:-%d", time.Now().UnixNano()))
	opts.SetDefaultPublishHandler(f)
	opts.SetConnectTimeout(time.Duration(60) * time.Second)

	//创建连接
	c := MQTT.NewClient(opts)

	if token := c.Connect(); token.WaitTimeout(time.Duration(60)*time.Second) && token.Wait() && token.Error() != nil {
		fail_nums++
		fmt.Printf("taskId:sub fail_nums:%d,error:%s \n", fail_nums, token.Error())
		return
	}
	token := c.Subscribe("test", 2, func(cl MQTT.Client, msg MQTT.Message) {
		//fmt.Printf("receive msg : %+v\n", msg)
		atomic.AddInt64(n, 1)
	})
	token.Wait()
}
func createTask(taskId int, wg *sync.WaitGroup) {
	defer wg.Done()
	opts := MQTT.NewClientOptions().AddBroker("tcp://127.0.0.1:1887").SetUsername("admin").SetPassword("123456")
	opts.SetClientID(fmt.Sprintf("go-simple-client:%d-%d", taskId, time.Now().UnixNano()))
	opts.SetDefaultPublishHandler(f)
	opts.SetConnectTimeout(time.Duration(60) * time.Second)

	//创建连接
	c := MQTT.NewClient(opts)

	if token := c.Connect(); token.WaitTimeout(time.Duration(60)*time.Second) && token.Wait() && token.Error() != nil {
		fail_nums++
		fmt.Printf("taskId:%d,fail_nums:%d,error:%s \n", taskId, fail_nums, token.Error())
		return
	}

	//每隔5秒向topic发送一条消息
	i := 0
	for i < 1000 {
		i++
		time.Sleep(time.Duration(10) * time.Millisecond)
		text := fmt.Sprintf("this is msg #%d! from task:%d", i, taskId)
		token := c.Publish("test", 2, false, text)
		token.Wait()
	}

	c.Disconnect(250)
	fmt.Println("task ok!!")
}
