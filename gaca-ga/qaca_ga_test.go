package gaca_ga

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestQAcaGa(t *testing.T) {
	cityNum := 50
	city := make([][2]float64, cityNum)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < cityNum; i++ {
		city[i][0] = r.Float64() * 600
		city[i][1] = r.Float64() * 600
	}
	fmt.Printf("%v", city)
	start := time.Now().UnixNano()
	qcacga := Build(city)
	qcacga.Run()
	end := time.Now().UnixNano()
	qcacga.Path()
	fmt.Println("总耗时：", float64(end-start)/1000000000)
}

func BenchmarkQAca_Run(b *testing.B) {
	cityNum := 50
	city := make([][2]float64, cityNum)
	for i := 0; i < cityNum; i++ {
		city[i][0] = float64(rand.Intn(100))
		city[i][1] = float64(rand.Intn(100))
	}
	fmt.Printf("%v", city)
	for i := 0; i < b.N; i++ {
		qcacga := Build(city)
		qcacga.Run()
	}
}
