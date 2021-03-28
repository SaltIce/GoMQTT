package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
)

var (
	ConstConf = MyConst{}
)

/*
获取程序运行路径
*/
func getCurrentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}
func init() {
	var err error
	err = readConst(&ConstConf, getCurrentDirectory()+"/mqtt_v5/config/const.yml")
	if err != nil {
		panic(err)
	}
	logger := MyConst{}
	err = readConst(&logger, getCurrentDirectory()+"/mqtt_v5/config/logger.yml")
	if err != nil {
		panic(err)
	}
	ConstConf.Logger = logger.Logger
}

/**
* 不能在"github.com/surgemq/surgemq/logger"文件中使用，会有循环依赖的
 */
//获取常量，防止循环依赖
func readConst(t *MyConst, filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("读取配置文件" + filePath + "出错")
		return err
	}
	//把yaml形式的字符串解析成struct类型 t保存初始数据
	err = yaml.Unmarshal(data, t)
	if err != nil {
		fmt.Println("解析配置文件" + filePath + "出错")
		return err
	}
	return nil
	//d, _ := yaml.Marshal(&t)
	//fmt.Println("看看解析文件后的数据 :\n", string(d))
}
