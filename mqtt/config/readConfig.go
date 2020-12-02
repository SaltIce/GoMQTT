package config

import (
	"Go-MQTT/mqtt/logger"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)
/**
* 不能在"github.com/surgemq/surgemq/logger"文件中使用，会有循环依赖的
 */
//获取常量，防止循环依赖
func ReadConst(t *MyConst, filePath string) (*MyConst, error) {
	data, err := ioutil.ReadFile(filePath)
	if err!= nil{
		logger.Error(err,"读取配置文件"+filePath+"出错")
		return nil,err
	}
	//把yaml形式的字符串解析成struct类型 t保存初始数据
	err = yaml.Unmarshal(data, t)
	if err!=nil{
		logger.Error(err,"解析配置文件"+filePath+"出错")
		return nil,err
	}
	return t,nil
	//d, _ := yaml.Marshal(&t)
	//fmt.Println("看看解析文件后的数据 :\n", string(d))
}
//获取权限认证配置，防止循环依赖
func ReadAuthConfig(t *MyAuth, filePath string) (*MyAuth, error) {
	data, err := ioutil.ReadFile(filePath)
	if err!= nil{
		logger.Error(err,"读取配置文件"+filePath+"出错")
		return nil,err
	}
	//把yaml形式的字符串解析成struct类型 t保存初始数据
	err = yaml.Unmarshal(data, t)
	if err!=nil{
		logger.Error(err,"解析配置文件"+filePath+"出错")
		return nil,err
	}
	return t,nil
	//d, _ := yaml.Marshal(&t)
	//fmt.Println("看看解析文件后的数据 :\n", string(d))
}
