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

package colong

import (
	"fmt"
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
	Pingack *Ackqueue
	Msgack  *Ackqueue
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

	this.id = string("")
	this.Pingack = newAckqueue(defaultQueueSize)
	this.Msgack = newAckqueue(defaultQueueSize)
	this.initted = true

	return nil
}
func (this *Session) ID() string {
	return this.id
}
