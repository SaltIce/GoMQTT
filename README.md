# Go-MQTT

#### 介绍
Go语言的MQTT，采用surgemq进行的二次开发，目前支持mqtt3 ，5版本正在完善

#### 软件架构
支持集群部署，目前集群方案为zk+quic
监听zk获取节点变化情况，自动与其它节点创建quic协议的udp连接
节点间目前只是转发消息过去，还未完全实现可靠的确认机制

#### 安装教程

1.  配置文件在 mqtt_v5/config/const.yml
2.  修改zk路径即可，各个节点的集群中的名称都是唯一的【节点信息】
3.  修改redis配置【共享主题】
3.  配置自己的cluster.hostip、brokerurl、wsbrokerurl, 还有cluster.name，全局唯一哦
4.  启动mqtt_v5\run\run.go即可

#### 存在问题

2.  集群节点间普通消息、sys消息、share消息确认机制未实现【只是简单的发一次过去测试功能，没有确保该消息唯一且一定达到，并且效率也要可以】
3.  集群客户端连接管理未完善
4.  节点下的客户端管理未实现
5.   客户端流控未实现
6.   主题别名未实现
7.   节点间断线导致的消息发送确认机制处理未实现
9.   sys消息转发会变为qos=0
10.  节点间的quic服务端与客户端断线时的消息处理
#### 参与贡献

1.  Fork 本仓库
2.  新建 Feat_xxx 分支
3.  提交代码
4.  新建 Pull Request