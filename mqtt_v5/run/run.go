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
	"Go-MQTT/mqtt_v5/comment"
	"Go-MQTT/mqtt_v5/config"
	_ "Go-MQTT/mqtt_v5/internal/nodediscover"
	"Go-MQTT/mqtt_v5/logger"
	"Go-MQTT/mqtt_v5/service"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
)

var (
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
	consts := config.ConstConf
	authenticator = consts.DefaultConst.Authenticator
	sessionsProvider = consts.DefaultConst.SessionsProvider
	topicsProvider = consts.DefaultConst.TopicsProvider
	cpuprofile = "F:\\Go_pro\\src\\Go-MQTT\\pprof_file\\cpu.txt"
	flag.IntVar(&keepAlive, "keepalive", comment.DefaultKeepAlive, "Keepalive (sec)")
	flag.IntVar(&connectTimeout, "connecttimeout", comment.DefaultConnectTimeout, "Connect Timeout (sec)")
	flag.IntVar(&ackTimeout, "acktimeout", comment.DefaultAckTimeout, "Ack Timeout (sec)")
	flag.IntVar(&timeoutRetries, "retries", comment.DefaultTimeoutRetries, "Timeout Retries")
	//权限认证的
	flag.StringVar(&authenticator, "auth", authenticator, "Authenticator Type")
	//下面两个的value要改都要改
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
	if strings.TrimSpace(config.ConstConf.BrokerUrl) != "" {
		mqttaddr = strings.TrimSpace(config.ConstConf.BrokerUrl)
	}
	wsAddr := ""
	if strings.TrimSpace(config.ConstConf.WsBrokerUrl) != "" {
		wsAddr = strings.TrimSpace(config.ConstConf.WsBrokerUrl)
	}
	BuffConfigInit()
	if len(wsAddr) > 0 || len(wssAddr) > 0 {
		AddWebsocketHandler("/mqtt", mqttaddr) // 将wsAddr的ws连接数据发到mqttaddr上

		/* start a plain websocket listener */
		if len(wsAddr) > 0 {
			go ListenAndServeWebsocket(wsAddr)
		}
		/* start a secure websocket listener */
		if len(wssAddr) > 0 && len(wssCertPath) > 0 && len(wssKeyPath) > 0 {
			go ListenAndServeWebsocketSecure(wssAddr, wssCertPath, wssKeyPath)
		}
	}

	/* create plain MQTT listener */
	err = svr.ListenAndServe(mqttaddr)
	if err != nil {
		logger.Errorf(err, "MQTT 启动异常错误 surgemq/main: %v", err)
	}

}

// buff 配置设置
func BuffConfigInit() {
	//buff := config.ConstConf.MyBuff
	//if buff.BufferSize > math.MaxInt64 {
	//	panic("config.ConstConf.MyBuff.BufferSize more than math.MaxInt64")
	//}
	//if buff.ReadBlockSize > math.MaxInt64 {
	//	panic("config.ConstConf.MyBuff.ReadBlockSize more than math.MaxInt64")
	//}
	//if buff.WriteBlockSize > math.MaxInt64 {
	//	panic("config.ConstConf.MyBuff.WriteBlockSize more than math.MaxInt64")
	//}
	//defaultBufferSize := buff.BufferSize
	//defaultReadBlockSize := buff.ReadBlockSize
	//defaultWriteBlockSize := buff.WriteBlockSize
	//service.BuffConfigInit(defaultBufferSize, defaultReadBlockSize, defaultWriteBlockSize)
}
