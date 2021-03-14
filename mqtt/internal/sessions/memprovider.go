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

package sessions

import (
	"Go-MQTT/mqtt/internal/config"
	"Go-MQTT/mqtt/internal/logger"
	"fmt"
	"strconv"
	"sync"
)

var (
	defaultSessionsProvider = ""
	serverSessionsProvider  = " "
	menSOpen                bool
)

func init() {
	co := config.MyConst{}
	consts, err := config.ReadConst(&co, "mqtt/config/const.yml")
	if err != nil {
		panic(err)
	}
	defaultSessionsProvider = consts.DefaultConst[0].Nodes[0].DefaultSessionsProvider
	serverSessionsProvider = consts.ServerConf.SessionsProvider
	if serverSessionsProvider == defaultSessionsProvider {
		menSOpen = true
	}
}

var _ SessionsProvider = (*memProvider)(nil)

func init() {
	if defaultSessionsProvider != "" {
		if menSOpen {
			Register(defaultSessionsProvider, NewMemProvider())
		}
	} else {
		fmt.Println("获取默认DefaultSessionsProvider失败")
	}
}

type memProvider struct {
	st map[string]*Session
	mu sync.RWMutex
}

func NewMemProvider() *memProvider {
	return &memProvider{
		st: make(map[string]*Session),
	}
}

func (this *memProvider) New(id string) (*Session, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.st[id] = &Session{id: id}
	logger.Info(strconv.Itoa(len(this.st)))
	return this.st[id], nil
}

func (this *memProvider) Get(id string) (*Session, error) {
	this.mu.RLock()
	defer this.mu.RUnlock()

	sess, ok := this.st[id]
	if !ok {
		return nil, fmt.Errorf("store/Get: No session found for key %s", id)
	}
	logger.Info(strconv.Itoa(len(this.st)))
	return sess, nil
}

func (this *memProvider) Del(id string) {
	this.mu.Lock()
	defer this.mu.Unlock()
	delete(this.st, id)
}

func (this *memProvider) Save(id string) error {
	return nil
}

func (this *memProvider) Count() int {
	return len(this.st)
}

func (this *memProvider) Close() error {
	this.st = make(map[string]*Session)
	return nil
}
