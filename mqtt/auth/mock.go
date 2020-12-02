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
	"Go-MQTT/mqtt/comment"
	"Go-MQTT/mqtt/config"
	"Go-MQTT/mqtt/logger"
	. "Go-MQTT/mysql"
)

type mockAuthenticator bool

var _ Authenticator = (*mockAuthenticator)(nil)

var (
	mockSuccessAuthenticator mockAuthenticator = true
	mockFailureAuthenticator mockAuthenticator = false
	defaultName = ""
	defaultPwd = ""
	openTestAuth = false
	mysqlPoolSize int
	isInitMysql = false
	mysqlUrl string
	mysqlAccount string
	mysqlPassword string
	mysqlDatabase string
)
//初始化数据库连接池
var dbChanPool chan DB//创建10连接的数据库连接池
func init() {
	co := config.MyConst{}
	consts , err := config.ReadConst(&co,"mqtt/config/const.yml")
	if err!=nil{
		panic(err)
	}
	defaultName = consts.MyAuth[0].Auth[0].DefaultName
	defaultPwd = consts.MyAuth[0].Auth[0].DefaultPwd
	openTestAuth = consts.MyAuth[0].Auth[0].Open
	mysqlUrl = consts.MyConst[0].Nodes[0].Mysql[0].MysqlUrl
	mysqlAccount = consts.MyConst[0].Nodes[0].Mysql[0].Account
	mysqlPassword = consts.MyConst[0].Nodes[0].Mysql[0].PassWord
	mysqlDatabase = consts.MyConst[0].Nodes[0].Mysql[0].DataBase
	if consts.ServerConf.Authenticator == consts.DefaultConst[0].Nodes[0].DefaultAuthenticator{
		isInitMysql = true
	}
	mysqlPoolSize = consts.MyConst[0].Nodes[0].Mysql[0].MysqlPoolSize
	if mysqlPoolSize<=0{
		mysqlPoolSize = 10 //要是格式不对，设置默认的
	}
	dbChanPool = make(chan DB, mysqlPoolSize)
}
func init() {
	if isInitMysql{
		for i := 0; i < 10; i++ {
			dd := DB{}
			db := dd.OpenLink("mysql", mysqlAccount+":"+mysqlPassword+"@tcp("+mysqlUrl+")/"+mysqlDatabase) //注意严格区分大小写 mq-mysql
			dd.Prepare(db, "select clientID,password from dev_user where account = ?")
			dbChanPool <- dd
		}
	}
}

func init() {
	if isInitMysql{
		Register(comment.DefaultAuthenticator, mockSuccessAuthenticator) //开启验证
		Register(comment.DefaultFailureAuthenticator, mockFailureAuthenticator) //关闭认证，直接失败，拒绝所有连接
		logger.Info("开启mysql账号认证")
	}else {
		logger.Info("未开启mysql账号认证")
	}
}

//权限认证
func (this mockAuthenticator) Authenticate(id string, cred interface{}) error {
	if this{
		//判断是否开启测试账号功能
		if openTestAuth{
			if id == defaultName && cred ==defaultPwd{
				logger.Info("默认账号登陆成功")
				//放行
				return nil
			}
		}
		if clientID ,ok := checkAuth(id,cred);ok{
			logger.Infof("mysql : 账号：%s，密码：%v，登陆-成功，clientID==%s",id,cred,clientID)
			return nil
		}else {
			logger.Infof("mysql : 账号：%s，密码：%v，登陆-失败",id,cred)
			return ErrAuthFailure
		}
	}
	logger.Info("当前未开启账号验证，取消一切连接。。。")
	//取消客户端登陆连接
	return ErrAuthFailure
}

/**
	验证身份，返回clientID
**/
func checkAuth(id string, cred interface{}) (string,bool) {
	if clientID ,ok := mysqlAuth(id,cred);ok{
		return clientID,true
	}
	return "",false
}
/**
	mysql账号认证，返回clientID
**/
func mysqlAuth(id string, cred interface{}) (string,bool) {
	db := <-dbChanPool
	defer func() {dbChanPool <- db}()
	clientID ,password,err := db.SelectClient(id)
	if  err != nil{
		return "",false
	}
	//这里可以添加MD5验证
	if password == cred.(string){
		return clientID,true
	}else {
		return "",false
	}

}