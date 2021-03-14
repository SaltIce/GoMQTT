package quic

import (
	"Go-MQTT/codec"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"strconv"
)

const addr = "localhost:9999"
const message = "ccccccccccccccccccccccd"

func QUIC_Client_run() {
	session, err := quic.DialAddr(addr, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"quic"}}, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	bg := context.Background()
	cancel, cancelFunc := context.WithCancel(bg)
	defer cancelFunc()
	stream, err := session.OpenStreamSync(cancel)
	if err != nil {
		fmt.Println(err)
		return
	}
	i := 0
	go func() {
		var data2 [65535]byte
		result := bytes.NewBuffer(nil)
		for {
			n, err := stream.Read(data2[0:])
			if err != nil {
				panic(err)
			}
			result.Write(data2[0:n])
			scanner := bufio.NewScanner(result)
			scanner.Split(codec.PacketSplitFunc)
			for scanner.Scan() {
				fmt.Println(string(scanner.Bytes()[6:]))
			}
		}
	}()
	for {
		//fmt.Printf("Client: Sending '%s'\n", message)
		bf := codec.Encode([]byte(message + strconv.Itoa(i)))
		_, err = stream.Write(bf.Bytes())

		if err != nil {
			fmt.Println(err)
			return
		}
		//buf := make([]byte, bf.Len())
		//_, err = io.ReadFull(stream, buf)
		//if err != nil {
		//	fmt.Println(err)
		//	return
		//}
		i++
		//time.Sleep(2 * time.Second)
	}
}
