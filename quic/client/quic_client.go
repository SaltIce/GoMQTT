// 用于连接其它节点
// 这个会有多个实例
// 与不同的集群节点会有不同的连接
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"net/url"
)

const addr = "localhost:9999"
const message = "ccccccccccccccccccccccd"

//例如，URI可以是“tcp://127.0.0.1:1883”。
func QUICClientRun(uri string) {
	u, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	session, err := quic.DialAddr(u.Host, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"quic"}}, nil)
	if err != nil {
		panic(err)
	}
	bg := context.Background()
	cancel, cancelFunc := context.WithCancel(bg)
	defer cancelFunc()
	stream, err := session.OpenStreamSync(cancel)
	if err != nil {
		fmt.Println(err)
		return
	}
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stream.Read(buf)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("Client: Got '%v'\n", buf[:n])
		}
	}()
	for {
		login := []byte{16, 72, 0, 4, 77, 81, 84, 84, 4, 238, 0, 60, 0, 32, 49, 100, 49, 100, 100, 49, 102, 56, 49, 52, 51, 52, 52, 55, 102, 100, 56, 49, 98, 57, 50, 99, 101, 99, 97, 100, 101, 100, 48, 56, 57, 100, 0, 6, 119, 105, 108, 108, 47, 49, 0, 3, 49, 49, 49, 0, 5, 97, 100, 109, 105, 110, 0, 6, 49, 50, 51, 52, 53, 54}
		_, err = stream.Write(login)
		if err != nil {
			fmt.Println(err)
			return
		}
		pub := []byte{48, 11, 0, 6, 48, 48, 47, 53, 47, 50, 97, 97, 97}
		_, err = stream.Write(pub)
		if err != nil {
			fmt.Println(err)
			return
		}

		//time.Sleep(12 * time.Millisecond)
	}
}
