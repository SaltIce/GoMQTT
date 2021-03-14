package quic

import (
	"Go-MQTT/codec"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"io"
	"math/big"
)

const saddr = "localhost:8765"

func QUIC_Server_Run() {
	listener, err := quic.ListenAddr(saddr, generateTLSConfig(), nil)
	if err != nil {
		fmt.Println(err)
	}
	bg := context.Background()
	cancel, cancelFunc := context.WithCancel(bg)
	defer cancelFunc()
	for {
		sess, err := listener.Accept(cancel)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(sess)
			go dealSession(cancel, sess)
		}
	}
}
func dealSession(ctx context.Context, sess quic.Session) {
	stream, err := sess.AcceptStream(ctx)
	if err != nil {
		panic(err)
	} else {
		result := bytes.NewBuffer(nil)
		var buf [65542]byte // 由于 标识数据包长度 的只有两个字节 故数据包最大为 2^16+4(魔数)+2(长度标识)
		for {
			n, err := stream.Read(buf[0:])
			result.Write(buf[0:n])
			if err != nil {
				if err == io.EOF {
					panic(errors.New("客户端主动断开连接"))
				} else {
					//logger.PError(err, "read err:")
					panic(err)
				}
			} else {
				scanner := bufio.NewScanner(result)
				scanner.Split(codec.PacketSplitFunc)
				for scanner.Scan() {
					fmt.Printf("Server: Got '%s'\n", string(scanner.Bytes()[6:]))
					_, err = stream.Write(codec.Encode(scanner.Bytes()[6:]).Bytes())
					if err != nil {
						panic(err)
					}
				}
			}
			result.Reset()
		}
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}, NextProtos: []string{"quic"}}
}
