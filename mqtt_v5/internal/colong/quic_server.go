package colong

import (
	"Go-MQTT/mqtt_v5/comment"
	"Go-MQTT/mqtt_v5/logger"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"io"
	"math/big"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"Go-MQTT/mqtt_v5/auth"
	"Go-MQTT/mqtt_v5/sessions"
	"Go-MQTT/mqtt_v5/topics"
)

var (
	ErrInvalidSubscriber error = errors.New("service: Invalid subscriber")
)
var SVC *ColongSvc

// Server is a library implementation of the MQTT server that, as best it can, complies
// with the MQTT 3.1 and 3.1.1 specs.
// Server是MQTT服务器的一个库实现，它尽其所能遵守
//使用MQTT 3.1和3.1.1规范。
type Server struct {
	// The number of seconds to keep the connection live if there's no data.
	// If not set then default to 5 mins.
	//如果没有数据，保持连接的秒数。
	//如果没有设置，则默认为5分钟。
	KeepAlive int

	// The number of seconds to wait for the CONNECT message before disconnecting.
	// If not set then default to 2 seconds.
	//在断开连接之前等待连接消息的秒数。
	//如果没有设置，则默认为2秒。
	ConnectTimeout int

	// The number of seconds to wait for any ACK messages before failing.
	// If not set then default to 20 seconds.
	//失败前等待ACK消息的秒数。
	//如果没有设置，则默认为20秒。
	AckTimeout int

	// The number of times to retry sending a packet if ACK is not received.
	// If no set then default to 3 retries.
	//如果没有收到ACK，重试发送数据包的次数。
	//如果没有设置，则默认为3次重试。
	TimeoutRetries int

	// Authenticator is the authenticator used to check username and password sent
	// in the CONNECT message. If not set then default to "mockSuccess".
	// Authenticator是验证器，用于检查发送的用户名和密码
	//在连接消息中。如果不设置，则默认为"mockSuccess"
	Authenticator string

	// SessionsProvider is the session store that keeps all the Session objects.
	// This is the store to check if CleanSession is set to 0 in the CONNECT message.
	// If not set then default to "mem".
	// SessionsProvider是保存所有会话对象的会话存储。
	//这是用于检查在连接消息中是否将清洗设置为0的存储。
	//如果没有设置，则默认为"mem"。
	SessionsProvider string

	// TopicsProvider is the topic store that keeps all the subscription topics.
	// If not set then default to "mem".
	// TopicsProvider是保存所有订阅主题的主题存储。
	//如果没有设置，则默认为"mem"。
	TopicsProvider string

	// authMgr is the authentication manager that we are going to use for authenticating
	// incoming connections
	// authMgr是我们将用于身份验证的认证管理器
	// 传入的连接
	authMgr *auth.Manager

	// sessMgr is the sessions manager for keeping track of the sessions
	// sessMgr是用于跟踪会话的会话管理器
	sessMgr *sessions.Manager

	// topicsMgr is the topics manager for keeping track of subscriptions
	// topicsMgr是跟踪订阅的主题管理器
	topicsMgr *topics.Manager

	// The quit channel for the server. If the server detects that this channel
	// is closed, then it's a signal for it to shutdown as well.
	//服务器的退出通道。如果服务器检测到该通道
	//是关闭的，那么它也是一个关闭的信号。
	quit chan struct{}

	ln quic.Listener

	// A list of services created by the server. We keep track of them so we can
	// gracefully shut them down if they are still alive when the server goes down.
	//服务器创建的服务列表。我们跟踪他们，这样我们就可以
	//当服务器宕机时，如果它们仍然存在，那么可以优雅地关闭它们。
	svcs []*ColongSvc

	// Mutex for updating svcs
	//用于更新svc的互斥锁
	mu sync.Mutex

	// A indicator on whether this server is running
	//指示服务器是否运行的指示灯
	running int32

	// A indicator on whether this server has already checked configuration
	//指示此服务器是否已检查配置
	configOnce sync.Once

	subs []interface{}
	qoss []byte

	pubFunc func(msg interface{}) error

	// 客户端
	Svc *ColongSvc
	// 客户端名称
	Name string
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
func (this *Server) ListenAndServe(uri string, pubFunc func(msg interface{}) error) error {
	defer atomic.CompareAndSwapInt32(&this.running, 1, 0)
	// 防止重复启动
	if !atomic.CompareAndSwapInt32(&this.running, 0, 1) {
		return fmt.Errorf("server/ListenAndServe: Server is already running")
	}
	bg := context.Background()
	this.pubFunc = pubFunc

	this.quit = make(chan struct{})

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	this.ln, err = quic.ListenAddr(u.Host, generateTLSConfig(), nil)
	if err != nil {
		return err
	}
	defer this.ln.Close()

	if err != nil {
		panic(err)
	}
	logger.Infof("集群节点服务器准备就绪: server is ready...version：%s", comment.ServerVersion)
	var tempDelay time.Duration // how long to sleep on accept failure 接受失败要睡多久，默认5ms，最大1s

	for {
		conn, err := this.ln.Accept(bg)

		if err != nil {
			// http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-this.quit: //关闭服务器
				return nil

			default:
			}

			// Borrowed from go1.3.3/src/pkg/net/http/server.go:1699
			// 暂时的错误处理
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
			svc, err := this.handleConnection(bg, conn)
			if err != nil {
				logger.Info(err.Error())
			} else {
				SVC = svc // 这一步是不是多余的
			}
		}()
	}
}

// 创建客户端的监听
func (this *Server) ListenAndClient(uri string) error {
	defer atomic.CompareAndSwapInt32(&this.running, 1, 0)
	// 防止重复启动
	if !atomic.CompareAndSwapInt32(&this.running, 0, 1) {
		return fmt.Errorf("server/ListenAndServe: Server is already running")
	}

	this.quit = make(chan struct{})
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	session, err := quic.DialAddr(u.Host, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"quic"}}, nil)
	if err != nil {
		return err
	}
	bg := context.Background()
	cancel, cancelFunc := context.WithCancel(bg)
	defer cancelFunc()
	stream, err := session.OpenStreamSync(cancel)
	if err != nil {
		return err
	}
	logger.Infof("集群节点连接成功")
	var tempDelay time.Duration // how long to sleep on accept failure 接受失败要睡多久，默认5ms，最大1s

	defer func() {
		if err != nil {
			// http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-this.quit: //关闭服务器
				return
			default:
			}

			// Borrowed from go1.3.3/src/pkg/net/http/server.go:1699
			// 暂时的错误处理
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				logger.Errorf(err, "link error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
			}
			return
		}
	}()
	login := []byte{16, 72, 0, 4, 77, 81, 84, 84, 4, 238, 0, 60, 0, 32, 49, 100, 49, 100, 100, 49, 102, 56, 49, 52, 51, 52, 52, 55, 102, 100, 56, 49, 98, 57, 50, 99, 101, 99, 97, 100, 101, 100, 48, 56, 57, 100, 0, 6, 119, 105, 108, 108, 47, 49, 0, 3, 49, 49, 49, 0, 5, 97, 100, 109, 105, 110, 0, 6, 49, 50, 51, 52, 53, 54}
	_, err = stream.Write(login)
	if err != nil {
		fmt.Println(err)
	}
	//pub := []byte{48, 11, 0, 6, 48, 48, 47, 53, 47, 50, 97, 97, 97}
	//_, err = stream.Write(pub)
	//if err != nil {
	//	fmt.Println(err)
	//}
	go func() {
		err = this.handleServerConnection(bg, stream)
		if err != nil {
			panic(err)
		}
	}()
	return err
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

// Close terminates the server by shutting down all the client connections and closing
// the listener. It will, as best it can, clean up after itself.
func (this *Server) Close() error {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf(err.(error), "%v 节点关闭连接失败", this.Name)
		}
	}()
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
func (this *Server) handleConnection(ctx context.Context, c quic.Session) (svc *ColongSvc, err error) {
	if c == nil {
		return nil, ErrInvalidConnectionType
	}

	conn, ok := c.AcceptStream(ctx)
	if ok != nil {
		return nil, ErrInvalidConnectionType
	}
	defer func() {
		if err != nil {
			_ = c.CloseWithError(quic.ErrorCode(110), "110")
		}
	}()
	// 1.阅读并解码信息从连接线上的ConnectMessage
	// 2.如果没有解码错误，则使用用户名和密码进行身份验证。
	//否则，就写一封电报。ConnackMessage与 合适的错误。
	// 3.如果身份验证成功，则创建一个新会话或 检索现有会话
	// 4.给电报写一封成功的信ConnackMessage消息
	//从连线中读取连接消息，如果错误，则检查它是否正确
	//一个连接错误。 如果是连接错误，请发回正确的连接错误
	//客户端。无论错误类型如何退出。
	conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(this.ConnectTimeout)))
	b := make([]byte, 15)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	id := base64.URLEncoding.EncodeToString(b)
	sess := &Session{id: id}
	if err = sess.Init(); err != nil {
		return nil, err
	}
	svc = &ColongSvc{
		id:     atomic.AddUint64(&gsvcid, 1),
		client: false,

		keepAlive:      this.KeepAlive,
		connectTimeout: this.ConnectTimeout,
		ackTimeout:     this.AckTimeout,
		timeoutRetries: this.TimeoutRetries,
		conn:           conn,
		sess:           sess,
	}
	if err := svc.start(this.pubFunc); err != nil {
		svc.stop()
		return nil, err
	}

	//this.mu.Lock()
	//this.svcs = append(this.svcs, svc)
	//this.mu.Unlock()

	logger.Infof("(%s) server/handleConnection: Connection established.", svc.cid())

	return svc, nil
}

func (this *Server) handleServerConnection(ctx context.Context, c quic.Stream) error {
	if c == nil {
		return ErrInvalidConnectionType
	}
	b := make([]byte, 15)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return err
	}
	id := base64.URLEncoding.EncodeToString(b)
	sess := &Session{id: id}
	if err := sess.Init(); err != nil {
		return err
	}
	svc := &ColongSvc{
		id:     atomic.AddUint64(&gsvcid, 1),
		client: false,

		keepAlive:      this.KeepAlive,
		connectTimeout: this.ConnectTimeout,
		ackTimeout:     this.AckTimeout,
		timeoutRetries: this.TimeoutRetries,
		conn:           c,
		sess:           sess,
	}
	this.Svc = svc
	go func() {
		for {
			select {
			case <-time.After(10 * time.Second):
				_, _ = svc.writeMessage(NewPingreqMessage())
			}
		}
	}()
	if err := svc.startC(this.pubFunc); err != nil {
		svc.stop()
		return err
	}

	//this.mu.Lock()
	//this.svcs = append(this.svcs, svc)
	//this.mu.Unlock()

	logger.Infof("(%s) server/handleConnection: Connection established.", svc.cid())

	return nil
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
		return
	})

	return err
}

func (this *Server) getSession(svc *ColongSvc) error {
	//如果cleanession设置为0，服务器必须恢复与
	//客户端基于当前会话的状态，由客户端识别
	//标识符。如果没有会话与客户端标识符相关联
	//服务器必须创建一个新的会话。
	//
	//如果cleanession设置为1，客户端和服务器必须丢弃任何先前的
	//创建一个新的session。这个会话持续的时间与网络connection。与此会话关联的状态数据绝不能在任何会话中重用
	//后续会话。

	return nil
}
