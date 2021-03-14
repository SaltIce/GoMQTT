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

package auth

import (
	"Go-MQTT/mqtt_v5/logger"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrAuthFailure          = errors.New("auth: Authentication failure")
	ErrAuthProviderNotFound = errors.New("auth: Authentication provider not found")

	providers = make(map[string]Authenticator)
)

type Authenticator interface {
	Authenticate(id string, cred interface{}) error
}

func Register(name string, provider Authenticator) {
	if provider == nil {
		panic("auth: Register provide is nil")
	}

	if _, dup := providers[name]; dup {
		panic("auth: Register called twice for provider " + name)
	}

	providers[name] = provider
	logger.Infof("Register AuthProvide：'%s' success，%T", name, provider)
}

func Unregister(name string) {
	delete(providers, name)
}

type Manager struct {
	rwm *sync.RWMutex
	p   Authenticator
}

func NewManager(providerName string) (*Manager, error) {
	p, ok := providers[providerName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provider %q", providerName)
	}

	return &Manager{p: p, rwm: &sync.RWMutex{}}, nil
}

func (this *Manager) Authenticate(id string, cred interface{}) error {
	this.rwm.RLock()
	defer this.rwm.RUnlock()
	return this.p.Authenticate(id, cred)
}

// 动态更新校验方式
func (this *Manager) UpdateAuth(providerName string) error {
	this.rwm.Lock()
	defer this.rwm.Unlock()
	p, ok := providers[providerName]
	if !ok {
		return fmt.Errorf("session: unknown provider %q", providerName)
	}
	this.p = p
	return nil
}
