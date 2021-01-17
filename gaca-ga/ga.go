package gaca_ga

import (
	"fmt"
	"time"
)

type GA interface {
	// 种群初始化函数 tag=true代表进行蚁群-遗传算法
	Init(tag bool)
	// 计算两个城市之间的距离
	Distance(aCity, bCity [2]float64) float64
	// 计算距离数组的最小值
	Min(min *[]float64) []float64
	// 计算某一个方案的路径长度，适应度函数为路线长度的倒数 *稍微耗时操作
	PathLen(bestResult []int) float64
	// 选择操作 *耗时操作
	Choice()
	// 交叉操作
	Cross()
	// 变异操作
	Mutation()
	// 逆转操作  *耗时操作
	Reverse()
	//计算两个父类的相似度
	StrSimilar(str1, str2 string) float64
}
type gaWrapper struct {
	g *ga
}

func (g *gaWrapper) Init(tag bool) {
	t := time.Now().UnixNano()
	g.g.Init(tag)
	fmt.Println("Init 耗时：", time.Now().UnixNano()-t)
}

func (g *gaWrapper) Distance(aCity, bCity [2]float64) float64 {
	t := time.Now().UnixNano()
	v := g.g.Distance(aCity, bCity)
	fmt.Println("Distance 耗时：", time.Now().UnixNano()-t)
	return v
}

func (g *gaWrapper) Min(min *[]float64) []float64 {
	t := time.Now().UnixNano()
	v := g.g.Min(min)
	fmt.Println("Min 耗时：", time.Now().UnixNano()-t)
	return v
}

func (g *gaWrapper) PathLen(bestResult []int) float64 {
	//t := time.Now().UnixNano()
	v := g.g.PathLen(bestResult)
	//fmt.Println("PathLen 耗时：", time.Now().UnixNano()-t)
	return v
}

func (g *gaWrapper) Choice() {
	t := time.Now().UnixNano()
	g.g.Choice()
	fmt.Println("Choice 耗时：", time.Now().UnixNano()-t)
}

func (g *gaWrapper) Cross() {
	t := time.Now().UnixNano()
	g.g.Cross()
	fmt.Println("Cross 耗时：", time.Now().UnixNano()-t)
}

func (g *gaWrapper) Mutation() {
	t := time.Now().UnixNano()
	g.g.Mutation()
	fmt.Println("Mutation 耗时：", time.Now().UnixNano()-t)
}

func (g *gaWrapper) Reverse() {
	t := time.Now().UnixNano()
	g.g.Reverse()
	fmt.Println("Reverse 耗时：", time.Now().UnixNano()-t)
}

func (g *gaWrapper) StrSimilar(str1, str2 string) float64 {
	t := time.Now().UnixNano()
	v := g.g.StrSimilar(str1, str2)
	fmt.Println("StrSimilar 耗时：", time.Now().UnixNano()-t)
	return v
}
