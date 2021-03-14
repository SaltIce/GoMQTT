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
	"Go-MQTT/mqtt_v5/comment"
	"Go-MQTT/mqtt_v5/config"
)

type noAuthenticator bool

var _ Authenticator = (*noAuthenticator)(nil)

var (
	memAuth noAuthenticator = true
)

// default auth的初始化
func defaultAuthInit() {
	consts := config.ConstConf.MyAuth
	if consts.Open {
		defaultName = consts.DefaultName
		defaultPwd = consts.DefaultPwd
	}
	openTestAuth = consts.Open
	DefaultConfig := config.ConstConf.DefaultConst
	if DefaultConfig.Authenticator == "" || DefaultConfig.Authenticator == comment.DefaultAuthenticator {
		// 默认不验证
		Register(comment.DefaultAuthenticator, NewDefaultAuth()) //开启默认验证
	}
}
func NewDefaultAuth() Authenticator {
	return &memAuth
}

//权限认证
func (this noAuthenticator) Authenticate(id string, cred interface{}) error {
	return nil
}
