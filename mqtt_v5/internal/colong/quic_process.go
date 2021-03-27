package colong

import (
	"Go-MQTT/mqtt_v5/logger"
	"errors"
	"fmt"
	"io"
	"reflect"
)

var (
	errDisconnect = errors.New("Disconnect")
)

// processor() reads messages from the incoming buffer and processes them
// processor()从传入缓冲区读取消息并处理它们
func (this *ColongSvc) processor() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Errorf(r.(error), fmt.Sprintf("(%s) Recovering from panic:", this.cid()))
		}

		this.wgStopped.Done()
		this.stop()
		logger.Debugf("(%s) Stopping processor", this.cid())
	}()

	logger.Debugf("(%s) Starting processor", this.cid())

	this.wgStarted.Done()

	for {
		// 1. Find out what message is next and the size of the message
		//了解接下来是什么消息以及消息的大小
		mtype, total, err := this.peekMessageSize()
		if err != nil {
			if err != io.EOF {
				logger.Errorf(err, "(%s) Error peeking next message size", this.cid())
			}
			return
		}

		msg, n, err := this.peekMessage(mtype, total)
		if err != nil {
			if err != io.EOF {
				logger.Errorf(err, "(%s) Error peeking next message: %v", this.cid(), err)
			}
			return
		}

		logger.Debugf("(%s) Received: %s", this.cid(), msg)

		this.inStat.increment(int64(n))
		// 5. Process the read message
		//处理读消息
		err = this.processIncoming(msg)
		if err != nil {
			if err != errDisconnect {
				logger.Errorf(err, "(%s) Error processing %s: %v", this.cid(), msg.Name(), err)
			} else {
				return
			}
		}

		// 7. We should commit the bytes in the buffer so we can move on
		// 我们应该提交缓冲区中的字节，这样我们才能继续
		_, err = this.in.ReadCommit(total)
		if err != nil {
			if err != io.EOF {
				logger.Errorf(err, "(%s) Error committing %d read bytes: %v", this.cid(), total, err)
			}
			return
		}

		// 7. Check to see if done is closed, if so, exit
		// 检查done是否关闭，如果关闭，退出
		if this.isDone() && this.in.Len() == 0 {
			return
		}

		if this.inStat.msgs%1000 == 0 {
			logger.Debugf("(%s) Going to process message %d", this.cid(), this.inStat.msgs)
		}
	}
}

// processor()从传入缓冲区读取消息并处理它们，客户端使用
func (this *ColongSvc) processorC() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Errorf(r.(error), fmt.Sprintf("(%s) Recovering from panic:", this.cid()))
		}

		this.wgStopped.Done()
		this.stop()
		logger.Debugf("(%s) Stopping processor", this.cid())
	}()

	logger.Debugf("(%s) Starting processor", this.cid())

	this.wgStarted.Done()

	for {
		// 1. Find out what message is next and the size of the message
		//了解接下来是什么消息以及消息的大小
		mtype, total, err := this.peekMessageSize()
		if err != nil {
			if err != io.EOF {
				logger.Errorf(err, "(%s) Error peeking next message size", this.cid())
			}
			return
		}

		msg, n, err := this.peekMessage(mtype, total)
		if err != nil {
			if err != io.EOF {
				logger.Errorf(err, "(%s) Error peeking next message: %v", this.cid(), err)
			}
			return
		}

		logger.Debugf("(%s) Received: %s", this.cid(), msg)

		this.inStat.increment(int64(n))
		// 5. Process the read message
		//处理读消息
		err = this.processIncomingC(msg)
		if err != nil {
			if err != errDisconnect {
				logger.Errorf(err, "(%s) Error processing %s: %v", this.cid(), msg.Name(), err)
			} else {
				return
			}
		}

		// 7. We should commit the bytes in the buffer so we can move on
		// 我们应该提交缓冲区中的字节，这样我们才能继续
		_, err = this.in.ReadCommit(total)
		if err != nil {
			if err != io.EOF {
				logger.Errorf(err, "(%s) Error committing %d read bytes: %v", this.cid(), total, err)
			}
			return
		}

		// 7. Check to see if done is closed, if so, exit
		// 检查done是否关闭，如果关闭，退出
		if this.isDone() && this.in.Len() == 0 {
			return
		}

		if this.inStat.msgs%1000 == 0 {
			logger.Debugf("(%s) Going to process message %d", this.cid(), this.inStat.msgs)
		}
	}
}

//集群其它节点发送来的mqtt消息的处理，作为服务端
func (this *ColongSvc) processIncoming(msg Message) error {
	var err error = nil
	switch msg := msg.(type) {
	// TODO 统一做PublishMessage和SysMessage消息确认机制
	case *PublishMessage: // 客户端发来的普通消息
		// For PUBLISH message, we should figure out what QoS it is and process accordingly
		// If QoS == 0, we should just take the next step, no ack required
		// If QoS == 1, we should send back PUBACK, then take the next step
		// If QoS == 2, we need to put it in the ack queue, send back PUBREC
		err = this.processPublish(msg) // 发送给当前节点的客户端
		// 答复该客户端来的该条消息
	case *SharePubMessage: // 客户端发来的共享消息
		// For PUBLISH message, we should figure out what QoS it is and process accordingly
		// If QoS == 0, we should just take the next step, no ack required
		// If QoS == 1, we should send back PUBACK, then take the next step
		// If QoS == 2, we need to put it in the ack queue, send back PUBREC
		err = this.processSharePublish(msg) // 发送给当前节点的客户端
		// 答复该客户端来的该条消息
	case *PingreqMessage: // 客户端发来的心跳
		// For PINGREQ message, we should send back PINGRESP
		resp := NewPingrespMessage()
		_, err = this.writeMessage(resp)
	case *DisconnectMessage: // 客户端发来的断开连接请求
		// For DISCONNECT message, we should quit
		return errDisconnect
	case *ConnectMessage: // 客户端发来的连接包
		cack := NewConnackMessage()
		_, err = this.writeMessage(cack)
	case *SysMessage: // 客户端发来的系统消息
		err = this.processSys(msg) // 发送给当前节点的客户端

	default:
		return fmt.Errorf("(%s) invalid message type %s.", this.cid(), msg.Name())
	}
	if err != nil {
		logger.Debugf("(%s) Error processing acked message: %v", this.cid(), err)
	}
	return err
}

//当前节点接收到连接的服务端发来的数据， 作为客户端使用
func (this *ColongSvc) processIncomingC(msg Message) error {
	var err error = nil
	switch msg := msg.(type) {
	// 作为客户端会收到的消息
	case *PingrespMessage: // 心跳响应
		// 接收到其它节点的心跳响应
		//this.sess.Pingack.Ack(msg)
		//this.processAcked(this.sess.Pingack) // 提醒处理
	case *DisconnectMessage: // 服务端主动关闭连接
		// For DISCONNECT message, we should quit
		// TODO 需要进一步处理断开连接
		return errDisconnect
	case *ConnackMessage: // 连接响应
		// 客户端连接服务端认证成功
		logger.Info("连接认证成功")
	default:
		return fmt.Errorf("(%s) invalid message type %s.", this.cid(), msg.Name())
	}
	if err != nil {
		logger.Debugf("(%s) Error processing acked message: %v", this.cid(), err)
	}
	return err
}

// For PUBLISH message, we should figure out what QoS it is and process accordingly
// If QoS == 0, we should just take the next step, no ack required
// If QoS == 1, we should send back PUBACK, then take the next step
// If QoS == 2, we need to put it in the ack queue, send back PUBREC
//对于发布消息，我们应该弄清楚它的QoS是什么，并相应地进行处理
//如果QoS == 0，我们应该采取下一步，不需要ack
//如果QoS == 1，我们应该返回PUBACK，然后进行下一步
//如果QoS == 2，我们需要将其放入ack队列中，发送回PUBREC
// 要发给当前节点的处理协程去处理
// 这是其它节点发来消息的处理
func (this *ColongSvc) processPublish(msg *PublishMessage) error {
	return this.pubFunc(msg)
}

// 处理其它节点发来的共享消息
func (this *ColongSvc) processSharePublish(msg *SharePubMessage) error {
	return this.shareFunc(msg)
}

// 处理其它节点发来的$sys消息
func (this *ColongSvc) processSys(msg *SysMessage) error {
	return this.sysFunc(msg)
}
func (this *ColongSvc) processAcked(ackq *Ackqueue) {
	for _, ackmsg := range ackq.Acked() {
		// Let's get the messages from the saved message byte slices.
		//让我们从保存的消息字节片获取消息。
		msg, err := ackmsg.Mtype.New()
		if err != nil {
			logger.Errorf(err, "process/processAcked: Unable to creating new %s message: %v", ackmsg.Mtype, err)
			continue
		}

		if _, err := msg.Decode(ackmsg.Msgbuf); err != nil {
			logger.Errorf(err, "process/processAcked: Unable to decode %s message: %v", ackmsg.Mtype, err)
			continue
		}

		ack, err := ackmsg.State.New()
		if err != nil {
			logger.Errorf(err, "process/processAcked: Unable to creating new %s message: %v", ackmsg.State, err)
			continue
		}

		if _, err := ack.Decode(ackmsg.Ackbuf); err != nil {
			logger.Errorf(err, "process/processAcked: Unable to decode %s message: %v", ackmsg.State, err)
			continue
		}

		//glog.Debugf("(%s) Processing acked message: %v", this.cid(), ack)

		// - PUBACK if it's QoS 1 message. This is on the client side.
		// - PUBREL if it's QoS 2 message. This is on the server side.
		// - PUBCOMP if it's QoS 2 message. This is on the client side.
		// - SUBACK if it's a subscribe message. This is on the client side.
		// - UNSUBACK if it's a unsubscribe message. This is on the client side.
		//如果是QoS 1消息，则返回。这是在客户端。
		//- PUBREL，如果它是QoS 2消息。这是在服务器端。
		//如果是QoS 2消息，则为PUBCOMP。这是在客户端。
		//- SUBACK如果它是一个订阅消息。这是在客户端。
		//- UNSUBACK如果它是一个取消订阅的消息。这是在客户端。
		switch ackmsg.State {
		case PINGRESP:
			logger.Debugf("process/processAcked: %s", ack)
			// If ack is PUBACK, that means the QoS 1 message sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是PUBACK，则表示此服务发送的QoS 1消息已获取
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PUBCOMP, that means the QoS 2 message sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack为PUBCOMP，则表示此服务发送的QoS 2消息已获得
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is SUBACK, that means the SUBSCRIBE message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是SUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is UNSUBACK, that means the SUBSCRIBE message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是UNSUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PINGRESP, that means the PINGREQ message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是PINGRESP，则表示此服务发送的PINGREQ消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。
			err = nil

		default:
			logger.Errorf(err, "(%s) Invalid ack message type %s.", this.cid(), ackmsg.State)
			continue
		}

		// Call the registered onComplete function
		if ackmsg.OnComplete != nil {
			onComplete, ok := ackmsg.OnComplete.(OnCompleteFunc)
			if !ok {
				logger.Errorf(nil, "process/processAcked: Error type asserting onComplete function: %v", reflect.TypeOf(ackmsg.OnComplete))
			} else if onComplete != nil {
				if err := onComplete(msg, ack, nil); err != nil {
					logger.Errorf(err, "process/processAcked: Error running onComplete(): %v", err)
				}
			}
		}
	}
}
