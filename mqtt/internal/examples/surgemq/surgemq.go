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

package main

import (
	"Go-MQTT/mqtt/internal/colong"
	"Go-MQTT/mqtt/internal/comment"
	"Go-MQTT/mqtt/internal/config"
	"Go-MQTT/mqtt/internal/logger"
	"Go-MQTT/mqtt/internal/service"
	"flag"
	"golang.org/x/net/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/pprof"
)

var (
	clusterEnabled   bool
	keepAlive        int
	connectTimeout   int
	ackTimeout       int
	timeoutRetries   int
	authenticator    string
	sessionsProvider string
	topicsProvider   string
	cpuprofile       string
	wsAddr           string // HTTPS websocket address eg. :8080
	wssAddr          string // HTTPS websocket address, eg. :8081
	wssCertPath      string // path to HTTPS public key
	wssKeyPath       string // path to HTTPS private key
)

func init() {
	co := config.MyConst{}
	consts, err := config.ReadConst(&co, "mqtt/config/const.yml")
	if err != nil {
		panic(err)
	}
	authenticator = consts.ServerConf.Authenticator
	sessionsProvider = consts.ServerConf.SessionsProvider
	topicsProvider = consts.ServerConf.TopicsProvider
	clusterEnabled = consts.Cluster.Enabled
}
func init() {
	flag.IntVar(&keepAlive, "keepalive", comment.DefaultKeepAlive, "Keepalive (sec)")
	flag.IntVar(&connectTimeout, "connecttimeout", comment.DefaultConnectTimeout, "Connect Timeout (sec)")
	flag.IntVar(&ackTimeout, "acktimeout", comment.DefaultAckTimeout, "Ack Timeout (sec)")
	flag.IntVar(&timeoutRetries, "retries", comment.DefaultTimeoutRetries, "Timeout Retries")
	//???????????????
	flag.StringVar(&authenticator, "auth", authenticator, "Authenticator Type")
	//???????????????value???????????????
	flag.StringVar(&sessionsProvider, "sessions", sessionsProvider, "Session Provider Type")
	flag.StringVar(&topicsProvider, "topics", topicsProvider, "Topics Provider Type")

	flag.StringVar(&cpuprofile, "cpuprofile", "", "CPU Profile Filename")
	flag.StringVar(&wsAddr, "wsaddr", "", "HTTP websocket address, eg. ':8080'")
	flag.StringVar(&wssAddr, "wssaddr", "", "HTTPS websocket address, eg. ':8081'")
	flag.StringVar(&wssCertPath, "wsscertpath", "", "HTTPS server public key file")
	flag.StringVar(&wssKeyPath, "wsskeypath", "", "HTTPS server private key file")
	flag.Parse()
}

func main() {
	svr := &service.Server{
		KeepAlive:        keepAlive,
		ConnectTimeout:   connectTimeout,
		AckTimeout:       ackTimeout,
		TimeoutRetries:   timeoutRetries,
		SessionsProvider: sessionsProvider,
		TopicsProvider:   topicsProvider,
		Authenticator:    authenticator,
	}

	var f *os.File
	var err error

	if cpuprofile != "" {
		f, err = os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, os.Kill)
	go func() {
		sig := <-sigchan
		logger.Infof("Existing due to trapped signal; %v", sig)

		if f != nil {
			logger.Info("Stopping profile")
			pprof.StopCPUProfile()
			f.Close()
		}

		svr.Close()

		os.Exit(0)
	}()

	mqttaddr := "tcp://:1883"

	if len(wsAddr) > 0 || len(wssAddr) > 0 {
		addr := "tcp://127.0.0.1:1889"
		AddWebsocketHandler("/mqtt", addr)
		/* start a plain websocket listener */
		if len(wsAddr) > 0 {
			logger.Info("here")
			go ListenAndServeWebsocket(wsAddr)
		}
		/* start a secure websocket listener */
		if len(wssAddr) > 0 && len(wssCertPath) > 0 && len(wssKeyPath) > 0 {
			go ListenAndServeWebsocketSecure(wssAddr, wssCertPath, wssKeyPath)
		}
	}

	//????????????
	go colong.Nets()

	/* create plain MQTT listener */
	err = svr.ListenAndServe(mqttaddr)
	if err != nil {
		logger.Errorf(err, "MQTT ?????????????????? surgemq/main: %v", err)
	}

}

func DefaultListenAndServeWebsocket() error {
	if err := AddWebsocketHandler("/mqtt", "127.0.0.1:1883"); err != nil {
		return err
	}
	return ListenAndServeWebsocket(":1234")
}

func AddWebsocketHandler(urlPattern string, uri string) error {
	logger.Debugf("AddWebsocketHandler urlPattern=%s, uri=%s", urlPattern, uri)
	u, err := url.Parse(uri)
	if err != nil {
		logger.Errorf(err, "surgemq/main: %v", err)
		return err
	}

	h := func(ws *websocket.Conn) {
		WebsocketTcpProxy(ws, u.Scheme, u.Host)
	}
	http.Handle(urlPattern, websocket.Handler(h))
	return nil
}

/* start a listener that proxies websocket <-> tcp */
func ListenAndServeWebsocket(addr string) error {
	return http.ListenAndServe(addr, nil)
}

/* starts an HTTPS listener */
func ListenAndServeWebsocketSecure(addr string, cert string, key string) error {
	return http.ListenAndServeTLS(addr, cert, key, nil)
}

/* copy from websocket to writer, this copies the binary frames as is */
func io_copy_ws(src *websocket.Conn, dst io.Writer) (int, error) {
	var buffer []byte
	count := 0
	for {
		err := websocket.Message.Receive(src, &buffer)
		if err != nil {
			return count, err
		}
		n := len(buffer)
		count += n
		i, err := dst.Write(buffer)
		if err != nil || i < 1 {
			return count, err
		}
	}
	return count, nil
}

/* copy from reader to websocket, this copies the binary frames as is */
func io_ws_copy(src io.Reader, dst *websocket.Conn) (int, error) {
	buffer := make([]byte, 2048)
	count := 0
	for {
		n, err := src.Read(buffer)
		if err != nil || n < 1 {
			return count, err
		}
		count += n
		err = websocket.Message.Send(dst, buffer[0:n])
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

/* handler that proxies websocket <-> unix domain socket */
func WebsocketTcpProxy(ws *websocket.Conn, nettype string, host string) error {
	client, err := net.Dial(nettype, host)
	if err != nil {
		return err
	}
	defer client.Close()
	defer ws.Close()
	chDone := make(chan bool)

	go func() {
		io_ws_copy(client, ws)
		chDone <- true
	}()
	go func() {
		io_copy_ws(ws, client)
		chDone <- true
	}()
	<-chDone
	return nil
}
