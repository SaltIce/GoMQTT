//// 用于接收其它节点的连接
package colong

//
//import (
//	"Go-MQTT/mqtt_v5/logger"
//	"Go-MQTT/mqtt_v5/sessions"
//	"errors"
//	"fmt"
//	"io"
//	"sync"
//	"sync/atomic"
//)
//
//var (
//	ErrInvalidConnectionType  error = errors.New("service: Invalid connection type")
//	ErrBufferNotReady         error = errors.New("service: buffer is not ready")
//	ErrBufferInsufficientData error = errors.New("service: buffer has insufficient data.") //缓冲区数据不足。
//)
//
//type (
//	//完成的回调方法
//	OnCompleteFunc func(msg, ack Message, err error) error
//	OnPublishFunc  func(msg *PublishMessage) error
//)
//
//type stat struct {
//	bytes int64 // bytes数量
//	msgs  int64 // 消息数量
//}
//
//func (this *stat) increment(n int64) {
//	atomic.AddInt64(&this.bytes, n)
//	atomic.AddInt64(&this.msgs, 1)
//}
//
//var (
//	gsvcid uint64 = 0
//)
//
//type ColongSvc struct {
//	// The ID of this service, it's not related to the Client ID, just a number that's
//	// incremented for every new service.
//	//这个服务的ID，它与客户ID无关，只是一个数字而已
//	//每增加一个新服务。
//	id uint64
//
//	// Is this a client or server. It's set by either Connect (client) or
//	// HandleConnection (server).
//	//这是客户端还是服务器?它是由Connect (client)或
//	// HandleConnection(服务器)。
//	// 用来表示该是服务端的还是客户端的
//	client bool
//
//	// The number of seconds to keep the connection live if there's no data.
//	// If not set then default to 5 mins.
//	//如果没有数据，保持连接有效的秒数。
//	//如果没有设置，则默认为5分钟。
//	keepAlive int
//
//	// The number of seconds to wait for the CONNACK message before disconnecting.
//	// If not set then default to 2 seconds.
//	//断开连接前等待CONNACK消息的秒数。
//	//如果没有设置，则默认为2秒。
//	connectTimeout int
//
//	// The number of seconds to wait for any ACK messages before failing.
//	// If not set then default to 20 seconds.
//	//在失败之前等待任何ACK消息的秒数。
//	//如果没有设置，则默认为20秒。
//	ackTimeout int
//
//	// The number of times to retry sending a packet if ACK is not received.
//	// If no set then default to 3 retries.
//	//如果没有收到ACK，重试发送数据包的次数。
//	//如果没有设置，则默认为3次重试。
//	timeoutRetries int
//
//	// Network connection for this service
//	//此服务的网络连接
//	conn io.Closer
//
//	// Session manager for tracking all the clients
//	//会话管理器，用于跟踪所有客户端
//	sessMgr *sessions.Manager
//
//	// sess is the session object for this MQTT session. It keeps track session variables
//	// such as ClientId, KeepAlive, Username, etc
//	// sess是这个MQTT会话的会话对象。它跟踪会话变量
//	//比如ClientId, KeepAlive，用户名等
//	sess *Session
//
//	// Wait for the various goroutines to finish starting and stopping
//	//等待各种goroutines完成启动和停止
//	wgStarted sync.WaitGroup
//	wgStopped sync.WaitGroup
//
//	// writeMessage mutex - serializes writes to the outgoing buffer.
//	// writeMessage互斥锁——序列化输出缓冲区的写操作。
//	wmu sync.Mutex
//
//	// Whether this is service is closed or not.
//	//这个服务是否关闭。
//	closed int64
//
//	// Quit signal for determining when this service should end. If channel is closed,
//	// then exit.
//	//退出信号，用于确定此服务何时结束。如果通道关闭，
//	//然后退出。
//	done chan struct{}
//
//	// Incoming data buffer. Bytes are read from the connection and put in here.
//	//输入数据缓冲区。从连接中读取字节并放在这里。
//	in *buffer
//
//	// Outgoing data buffer. Bytes written here are in turn written out to the connection.
//	//输出数据缓冲区。这里写入的字节依次写入连接。用于确认接收到的message
//	out *buffer
//
//	//onpub方法，将其添加到主题订阅方列表
//	// processSubscribe()方法。当服务器完成一个发布消息的ack循环时
//	//它将调用订阅者，也就是这个方法。
//	//对于服务器，当这个方法被调用时，它意味着有一个消息
//	//应该发布到连接另一端的客户端。所以我们
//	//将调用publish()发送消息。
//	onpub OnPublishFunc
//
//	inStat  stat // 输入的记录
//	outStat stat // 输出的记录
//
//	intmp  []byte
//	outtmp []byte
//
//	subs  []interface{}
//	qoss  []byte
//	rmsgs []*PublishMessage
//
//	pubFunc func(msg interface{}) error
//}
//
//func (this *ColongSvc) start(pubFunc func(msg interface{}) error) error {
//	var err error
//	this.pubFunc = pubFunc
//	// Create the incoming ring buffer
//	this.in, err = newBuffer(defaultBufferSize)
//	if err != nil {
//		return err
//	}
//
//	// Create the outgoing ring buffer
//	this.out, err = newBuffer(defaultBufferSize)
//	if err != nil {
//		return err
//	}
//
//	// Processor is responsible for reading messages out of the buffer and processing
//	// them accordingly.
//	//处理器负责从缓冲区读取消息并进行处理
//	//他们。
//	this.wgStarted.Add(1)
//	this.wgStopped.Add(1)
//	go this.processor()
//
//	// Receiver is responsible for reading from the connection and putting data into
//	// a buffer.
//	//接收端负责从连接中读取数据并将数据放入
//	//一个缓冲区。
//	this.wgStarted.Add(1)
//	this.wgStopped.Add(1)
//	go this.receiver()
//
//	// Sender is responsible for writing data in the buffer into the connection.
//	//发送方负责将缓冲区中的数据写入连接。
//	this.wgStarted.Add(1)
//	this.wgStopped.Add(1)
//	go this.sender()
//
//	// Wait for all the goroutines to start before returning
//	this.wgStarted.Wait()
//
//	return nil
//}
//
//// FIXME: The order of closing here causes panic sometimes. For example, if receiver
//// calls this, and closes the buffers, somehow it causes buffer.go:476 to panid.
//func (this *ColongSvc) stop() {
//	defer func() {
//		// Let's recover from panic
//		if r := recover(); r != nil {
//			logger.Errorf(nil, "(%s) Recovering from panic: %v", this.cid(), r)
//		}
//	}()
//
//	doit := atomic.CompareAndSwapInt64(&this.closed, 0, 1)
//	if !doit {
//		return
//	}
//
//	// Close quit channel, effectively telling all the goroutines it's time to quit
//	if this.done != nil {
//		logger.Debugf("(%s) closing this.done", this.cid())
//		close(this.done)
//	}
//
//	// Close the network connection
//	if this.conn != nil {
//		logger.Debugf("(%s) closing this.conn", this.cid())
//		this.conn.Close()
//	}
//
//	this.in.Close()
//	this.out.Close()
//
//	// Wait for all the goroutines to stop.
//	this.wgStopped.Wait()
//
//	//打印该客户端生命周期内的接收字节与消息条数、发送字节与消息条数
//	logger.Debugf("(%s) Received %d bytes in %d messages.", this.cid(), this.inStat.bytes, this.inStat.msgs)
//	logger.Debugf("(%s) Sent %d bytes in %d messages.", this.cid(), this.outStat.bytes, this.outStat.msgs)
//
//	this.conn = nil
//	this.in = nil
//	this.out = nil
//}
//
//func (this *ColongSvc) publish(msg *PublishMessage, onComplete OnCompleteFunc) error {
//
//	_, err := this.writeMessage(msg)
//	if err != nil {
//		return fmt.Errorf("(%s) Error sending %s message: %v", this.cid(), msg.Name(), err)
//	}
//	// TODO 等待这个消息确认
//	return this.sess.Msgack.Wait(msg, onComplete)
//}
//
//func (this *ColongSvc) ping(onComplete OnCompleteFunc) error {
//	msg := NewPingreqMessage()
//
//	_, err := this.writeMessage(msg)
//	if err != nil {
//		return fmt.Errorf("(%s) Error sending %s message: %v", this.cid(), msg.Name(), err)
//	}
//
//	return this.sess.Pingack.Wait(msg, onComplete)
//}
//
//func (this *ColongSvc) isDone() bool {
//	select {
//	case <-this.done:
//		return true
//
//	default:
//	}
//
//	return false
//}
//
//func (this *ColongSvc) cid() string {
//	return fmt.Sprintf("%d/%s", this.id, this.sess.ID())
//}
