package colong

import (
	"fmt"
	uuid "github.com/satori/go.uuid"
	"sync"
)

const (
	// Queue size for the ack queue
	//队列的队列大小
	defaultQueueSize = 64
)

// 会话
type Session struct {
	// Ack queue for outgoing PINGREQ messages
	//用于发送PINGREQ消息的Ack队列
	Pingack        *Ackqueue
	Msgack         *Ackqueue // SharePub的确认等待队列
	ShareReqMsgAck *Ackqueue // ShareReq的确认等待队列
	SysMsgack      *Ackqueue // 系统消息$sys/ 的ack等待
	// Initialized?
	initted bool
	// Serialize access to this session
	//序列化对该会话的访问锁
	mu sync.Mutex
	id string
}

func (this *Session) Init() error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.initted {
		return fmt.Errorf("Session already initialized")
	}

	this.id = uuid.NewV4().String()
	this.Pingack = newAckqueue(defaultQueueSize)
	this.Msgack = newAckqueue(defaultQueueSize * (2 << 10))         // 节点间连接并不多，可以适当调大点
	this.ShareReqMsgAck = newAckqueue(defaultQueueSize * (2 << 10)) // 节点间连接并不多，可以适当调大点
	this.SysMsgack = newAckqueue(defaultQueueSize * (2 << 2))       // 系统消息并不会太多
	this.initted = true

	return nil
}
func (this *Session) ID() string {
	return this.id
}
