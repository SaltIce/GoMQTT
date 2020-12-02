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

package service

import (
	"errors"
	"fmt"
	"Go-MQTT/mqtt/comment"
	"Go-MQTT/mqtt/logger"
	"io"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"Go-MQTT/mqtt/message"
	"Go-MQTT/mqtt/sessions"
	"Go-MQTT/mqtt/topics"
	"Go-MQTT/mqtt/auth"
)

var (
	ErrInvalidConnectionType  error = errors.New("service: Invalid connection type")
	ErrInvalidSubscriber      error = errors.New("service: Invalid subscriber")
	ErrBufferNotReady         error = errors.New("service: buffer is not ready")
	ErrBufferInsufficientData error = errors.New("service: buffer has insufficient data.")
)
var SVC *service

// Server is a library implementation of the MQTT server that, as best it can, complies
// with the MQTT 3.1 and 3.1.1 specs.
type Server struct {
	// The number of seconds to keep the connection live if there's no data.
	// If not set then default to 5 mins.
	KeepAlive int

	// The number of seconds to wait for the CONNECT message before disconnecting.
	// If not set then default to 2 seconds.
	ConnectTimeout int

	// The number of seconds to wait for any ACK messages before failing.
	// If not set then default to 20 seconds.
	AckTimeout int

	// The number of times to retry sending a packet if ACK is not received.
	// If no set then default to 3 retries.
	TimeoutRetries int

	// Authenticator is the authenticator used to check username and password sent
	// in the CONNECT message. If not set then default to "mockSuccess".
	Authenticator string

	// SessionsProvider is the session store that keeps all the Session objects.
	// This is the store to check if CleanSession is set to 0 in the CONNECT message.
	// If not set then default to "mem".
	SessionsProvider string

	// TopicsProvider is the topic store that keeps all the subscription topics.
	// If not set then default to "mem".
	TopicsProvider string

	// authMgr is the authentication manager that we are going to use for authenticating
	// incoming connections
	authMgr *auth.Manager

	// sessMgr is the sessions manager for keeping track of the sessions
	sessMgr *sessions.Manager

	// topicsMgr is the topics manager for keeping track of subscriptions
	topicsMgr *topics.Manager

	// The quit channel for the server. If the server detects that this channel
	// is closed, then it's a signal for it to shutdown as well.
	//服务器的退出通道。如果服务器检测到该通道
	//是关闭的，那么它也是一个关闭的信号。
	quit chan struct{}

	ln net.Listener

	// A list of services created by the server. We keep track of them so we can
	// gracefully shut them down if they are still alive when the server goes down.
	//服务器创建的服务列表。我们跟踪他们，这样我们就可以
	//当服务器宕机时，如果它们仍然存在，那么可以优雅地关闭它们。
	svcs []*service

	// Mutex for updating svcs
	mu sync.Mutex

	// A indicator on whether this server is running
	running int32

	// A indicator on whether this server has already checked configuration
	//指示此服务器是否已检查配置
	configOnce sync.Once

	subs []interface{}
	qoss []byte
}

// ListenAndServe listents to connections on the URI requested, and handles any
// incoming MQTT client sessions. It should not return until Close() is called
// or if there's some critical error that stops the server from running. The URI
// supplied should be of the form "protocol://host:port" that can be parsed by
// url.Parse(). For example, an URI could be "tcp://0.0.0.0:1883".
// ListenAndServe监听请求的URI上的连接，并处理任何连接
//传入的MQTT客户机会话。 在调用Close()之前，它不应该返回
//或者有一些关键的错误导致服务器停止运行。 URI
//提供的格式应该是“protocol://host:port”，可以通过它进行解析
// url.Parse ()。
//例如，URI可以是“tcp://0.0.0.0:1883”。
func (this *Server) ListenAndServe(uri string) error {
	defer atomic.CompareAndSwapInt32(&this.running, 1, 0)

	if !atomic.CompareAndSwapInt32(&this.running, 0, 1) {
		return fmt.Errorf("server/ListenAndServe: Server is already running")
	}

	this.quit = make(chan struct{})

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	this.ln, err = net.Listen(u.Scheme, u.Host)
	if err != nil {
		return err
	}
	defer this.ln.Close()

	logger.Infof("\n"+
		"\\***\n"+
		"*  _ooOoo_\n"+
		"* o8888888o\n"+
		"*'88' . '88' \n"+
		"* (| -_- |)\n"+
		"*  O\\ = /O\n"+
		"* ___/`---'\\____\n"+
		"* .   ' \\| |// `.\n"+
		"* / \\\\||| : |||// \\\n*"+
		" / _||||| -:- |||||- \\\n*"+
		" | | \\\\\\ - /// | |\n"+
		"* | \\_| ''\\---/'' | |\n"+
		"* \\ .-\\__ `-` ___/-. /\n"+
		"* ___`. .' /--.--\\ `. . __\n"+
		"* .'' '< `.___\\_<|>_/___.' >''''.\n"+
		"* | | : `- \\`.;`\\ _ /`;.`/ - ` : | |\n"+
		"* \\ \\ `-. \\_ __\\ /__ _/ .-` / /\n"+
		"* ======`-.____`-.___\\_____/___.-`____.-'======\n"+
		"* `=---='\n"+
		"*          .............................................\n"+
		"*           佛曰：bug泛滥，我已瘫痪！\n"+
		"*/\n"+"/****\n"+
		"* ░▒█████▒█    ██  ▄████▄ ██ ▄█▀       ██████╗   ██╗   ██╗ ██████╗\n"+
		"* ▓█     ▒ ██    ▓█ ▒▒██▀ ▀█  ██▄█▒        ██╔══██╗ ██║   ██║ ██╔════╝\n"+
		"* ▒████ ░▓█   ▒██░▒▓█    ▄   ██▄░         ██████╔╝ ██║   ██║ ██║  ███╗\n"+
		"* ░▓█▒    ░▓▓   ░██░▒▓▓▄ ▄█  ▒▓██▄       ██╔══██╗ ██║   ██║ ██║   ██║\n"+
		"* ░▒█░    ▒▒█████▓ ▒ ▓███▀  ░▒██▒ █▄     ██████╔╝╚██████╔╝╚██████╔╝\n"+
		"*  ▒ ░    ░▒▓▒ ▒ ▒ ░ ░▒ ▒  ░▒ ▒▒ ▓▒       ╚═════╝      ╚═════╝    ╚═════╝\n"+
		"*  ░      ░░▒░ ░ ░   ░  ▒   ░ ░▒ ▒░\n"+
		"*  ░ ░     ░░░ ░ ░ ░        ░ ░░ ░\n"+
		"*             ░     ░ ░      ░  ░\n"+
		"*/"+
		"服务器准备就绪: server is ready...version：%s", comment.ServerVersion)
	var tempDelay time.Duration // how long to sleep on accept failure

	for {
		conn, err := this.ln.Accept()

		if err != nil {
			// http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-this.quit: //关闭服务器
				return nil

			default:
			}

			// Borrowed from go1.3.3/src/pkg/net/http/server.go:1699
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				logger.Errorf(err, "Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}

		go func() {
			svc, err := this.handleConnection(conn)
			if err != nil {
				panic(err)
			} else {
				SVC = svc
			}
		}()
	}
}

// Publish sends a single MQTT PUBLISH message to the server. On completion, the
// supplied OnCompleteFunc is called. For QOS 0 messages, onComplete is called
// immediately after the message is sent to the outgoing buffer. For QOS 1 messages,
// onComplete is called when PUBACK is received. For QOS 2 messages, onComplete is
// called after the PUBCOMP message is received.
func (this *Server) Publish(msg *message.PublishMessage, onComplete OnCompleteFunc) error {
	if err := this.checkConfiguration(); err != nil {
		return err
	}

	if msg.Retain() {
		if err := this.topicsMgr.Retain(msg); err != nil {
			logger.Errorf(err, "Error retaining message: %v", err)
		}
	}

	if err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &this.subs, &this.qoss); err != nil {
		return err
	}

	msg.SetRetain(false)

	//glog.Debugf("(server) Publishing to topic %q and %d subscribers", string(msg.Topic()), len(this.subs))
	for _, s := range this.subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				logger.Info("Invalid onPublish Function")
			} else {
				(*fn)(msg)
			}
		}
	}

	return nil
}

// Close terminates the server by shutting down all the client connections and closing
// the listener. It will, as best it can, clean up after itself.
func (this *Server) Close() error {
	// By closing the quit channel, we are telling the server to stop accepting new
	// connection.
	close(this.quit)

	// We then close the net.Listener, which will force Accept() to return if it's
	// blocked waiting for new connections.
	this.ln.Close()

	for _, svc := range this.svcs {
		logger.Infof("Stopping service %d", svc.id)
		svc.stop()
	}

	if this.sessMgr != nil {
		this.sessMgr.Close()
	}

	if this.topicsMgr != nil {
		this.topicsMgr.Close()
	}

	return nil
}

// HandleConnection is for the broker to handle an incoming connection from a client
// HandleConnection用于代理处理来自客户机的传入连接
func (this *Server) handleConnection(c io.Closer) (svc *service, err error) {
	if c == nil {
		return nil, ErrInvalidConnectionType
	}

	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	//这个是配置各种钩子，比如账号认证钩子
	err = this.checkConfiguration()
	if err != nil {
		return nil, err
	}

	conn, ok := c.(net.Conn)
	if !ok {
		return nil, ErrInvalidConnectionType
	}

	// To establish a connection, we must
	// 1. Read and decode the message.ConnectMessage from the wire
	// 2. If no decoding errors, then authenticate using username and password.
	//    Otherwise, write out to the wire message.ConnackMessage with
	//    appropriate error.
	// 3. If authentication is successful, then either create a new session or
	//    retrieve existing session
	// 4. Write out to the wire a successful message.ConnackMessage message

	// Read the CONNECT message from the wire, if error, then check to see if it's
	// a CONNACK error. If it's CONNACK error, send the proper CONNACK error back
	// to client. Exit regardless of error type.
	//要建立联系，我们必须
	// 1。
	//阅读并解码信息从连接线上的ConnectMessage
	// 2。
	//如果没有解码错误，则使用用户名和密码进行身份验证。
	//否则，就写一封电报。ConnackMessage与 合适的错误。
	// 3。如果身份验证成功，则创建一个新会话或 检索现有会话
	// 4。
	//给电报写一封成功的信ConnackMessage消息
	//从连线中读取连接消息，如果错误，则检查它是否正确
	//一个连接错误。 如果是连接错误，请发回正确的连接错误
	//客户端。无论错误类型如何退出。
	conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(this.ConnectTimeout)))

	resp := message.NewConnackMessage()

	req, err := getConnectMessage(conn)
	if err != nil {
		if cerr, ok := err.(message.ConnackCode); ok {
			//glog.Debugf("request   message: %s\nresponse message: %s\nerror           : %v", mreq, resp, err)
			logger.Infof("request   message: %s\nresponse message: %s\nerror   : %v", nil, resp, err)
			resp.SetReturnCode(cerr)
			resp.SetSessionPresent(false)
			writeMessage(conn, resp)
		}
		return nil, err
	}

	// Authenticate the user, if error, return error and exit
	//登陆认证
	if err = this.authMgr.Authenticate(string(req.Username()), string(req.Password())); err != nil {
		//登陆失败日志，断开连接
		resp.SetReturnCode(message.ErrBadUsernameOrPassword)
		resp.SetSessionPresent(false)
		writeMessage(conn, resp)
		return nil, err
	}

	if req.KeepAlive() == 0 {
		//设置默认的keepalive数
		req.SetKeepAlive(minKeepAlive)
	}
	svc = &service{
		id:     atomic.AddUint64(&gsvcid, 1),
		client: false,

		keepAlive:      int(req.KeepAlive()),
		connectTimeout: this.ConnectTimeout,
		ackTimeout:     this.AckTimeout,
		timeoutRetries: this.TimeoutRetries,

		conn:      conn,
		sessMgr:   this.sessMgr,
		topicsMgr: this.topicsMgr,
	}
	err = this.getSession(svc, req, resp)
	if err != nil {
		return nil, err
	}

	resp.SetReturnCode(message.ConnectionAccepted)

	if err = writeMessage(c, resp); err != nil {
		return nil, err
	}

	svc.inStat.increment(int64(req.Len()))
	svc.outStat.increment(int64(resp.Len()))

	if err := svc.start(); err != nil {
		svc.stop()
		return nil, err
	}

	//this.mu.Lock()
	//this.svcs = append(this.svcs, svc)
	//this.mu.Unlock()

	logger.Infof("(%s) server/handleConnection: Connection established.", svc.cid())

	return svc, nil
}

func (this *Server) checkConfiguration() error {
	var err error

	this.configOnce.Do(func() {
		if this.KeepAlive == 0 {
			this.KeepAlive = comment.DefaultKeepAlive
		}

		if this.ConnectTimeout == 0 {
			this.ConnectTimeout = comment.DefaultConnectTimeout
		}

		if this.AckTimeout == 0 {
			this.AckTimeout = comment.DefaultAckTimeout
		}

		if this.TimeoutRetries == 0 {
			this.TimeoutRetries = comment.DefaultTimeoutRetries
		}

		if this.Authenticator == "" {
			logger.Info("这里缺少了权限认证钩子。。。")
			this.Authenticator = comment.DefaultAuthenticator
		}

		this.authMgr, err = auth.NewManager(this.Authenticator)
		if err != nil {
			return
		}

		if this.SessionsProvider == "" {
			this.SessionsProvider = comment.DefaultSessionsProvider
		}

		this.sessMgr, err = sessions.NewManager(this.SessionsProvider)
		if err != nil {
			logger.Error(err, "这里有错误。。。")
			return
		}

		if this.TopicsProvider == "" {
			this.TopicsProvider = comment.DefaultTopicsProvider
		}

		this.topicsMgr, err = topics.NewManager(this.TopicsProvider)

		return
	})

	return err
}

func (this *Server) getSession(svc *service, req *message.ConnectMessage, resp *message.ConnackMessage) error {
	// If CleanSession is set to 0, the server MUST resume communications with the
	// client based on state from the current session, as identified by the client
	// identifier. If there is no session associated with the client identifier the
	// server must create a new session.
	//
	// If CleanSession is set to 1, the client and server must discard any previous
	// session and start a new one. This session lasts as long as the network c
	// onnection. State data associated with this session must not be reused in any
	// subsequent session.

	var err error

	// Check to see if the client supplied an ID, if not, generate one and set
	// clean session.
	if len(req.ClientId()) == 0 {
		req.SetClientId([]byte(fmt.Sprintf("internalclient%d", svc.id)))
		req.SetCleanSession(true)
	}

	cid := string(req.ClientId())

	// If CleanSession is NOT set, check the session store for existing session.
	// If found, return it.
	if !req.CleanSession() {
		if svc.sess, err = this.sessMgr.Get(cid); err == nil {
			resp.SetSessionPresent(true)

			if err := svc.sess.Update(req); err != nil {
				return err
			}
		}
	}

	//如果没有session则创建一个
	// If CleanSession, or no existing session found, then create a new one
	if svc.sess == nil {
		if svc.sess, err = this.sessMgr.New(cid); err != nil {
			return err
		}

		resp.SetSessionPresent(false)

		if err := svc.sess.Init(req); err != nil {
			return err
		}
	}

	return nil
}
