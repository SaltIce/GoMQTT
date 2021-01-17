package gaca_ga

import (
	"fmt"
	"math"
	"time"
)

type qAca struct {
	time2      int64
	y          float64      //γ 表示量子信息素启发因子，表示 i 到 j 路径上量子态概率幅的相对重要性
	p          float64      //ρ 是信息素的蒸发率，有 0 < ρ < 1 ，参数 ρ 的作用是避免信息素的无限积累,信息素的蒸发根据后面的公式执行 τij = (1 - ρ)τij
	date       float64      //旋转角度  date(θ) 的大小直接影响到算法的收敛性
	maxDate    float64      // 最大旋转角度
	minDate    float64      // 最小旋转角度
	popNum     int          //种群数量
	cityNum    int          //城市数量
	iterNum    int          //迭代次数
	tao        [][]float64  //全局信息素
	distance   [][]float64  //全局距离矩阵
	bestTour   []int        //求解的最佳路径
	tourB      [][]int      //求解的最佳路径矩阵 选的路径为1，没选为0
	bestLength float64      //求的最优解的长度
	ants       []QAnt       //定义蚂蚁群
	cityP      [][2]float64 //城市位置信息
	ins        int
	gaMain     *GaMain
}

func Build(city [][2]float64) QAca {
	SIZE_POP = len(city)
	cityNum := SIZE_POP
	popNum := POP_NUM
	iterNum := ITER_NUM
	return newQAca(popNum, cityNum, iterNum, city, NewGaMain())
}
func newQAca(popNum, cityNum, iterNum int, city [][2]float64, ga *GaMain) QAca {
	return &qAca{
		time2:      0,
		y:          0.5,
		p:          0.7,
		date:       0.4,
		maxDate:    0.4,
		minDate:    0.005,
		popNum:     popNum,
		cityNum:    cityNum,
		iterNum:    iterNum,
		tao:        initTao(cityNum),
		distance:   initDistance(city, cityNum),
		bestTour:   make([]int, cityNum+1),
		tourB:      nil,
		bestLength: math.MaxFloat64,
		ants:       initPop(popNum, cityNum),
		cityP:      city,
		ins:        0,
		gaMain:     ga,
	}
}
func (q *qAca) Run() {
	start := time.Now().UnixNano()
	MAX_TAG := (q.iterNum / 10) * q.popNum
	//当MAX_TAG次循环没有出现最优解时，启动一次量子交叉
	crossTag := 0
	for runtimes := 0; runtimes < q.iterNum; runtimes++ {
		//重新随机设置蚂蚁
		for i := 0; i < q.popNum; i++ {
			q.ants[i].RandomSelectCity(q.cityNum, 0)
		}
		//每次迭代，所有蚂蚁都要跟新一遍，走一遍
		//每一只蚂蚁移动的过程
		for i := 0; i < q.popNum; i++ {
			for j := 1; j < q.cityNum; j++ {
				//每只蚂蚁的城市规划
				q.ants[i].SelectNextCity(j, &q.tao, &q.distance)
			}
			//计算蚂蚁获得的路径长度,这里把起点添加进去了，可以回到起点
			q.ants[i].CalTourLength(&q.distance)
			if q.ants[i].tourLength < q.bestLength {
				//保留最优路径
				q.bestLength = q.ants[i].tourLength
				//runtimes仅代表最大循环次数，但是只有当，有新的最优路径的时候才会显示下列语句。
				//如果后续没有更优解（收敛），则最后直接输出。
				for j := 0; j < q.cityNum; j++ {
					//更新全局最优路径
					q.bestTour[j] = q.ants[i].tour[j]
				}
				//更新全局最优路径矩阵信息
				q.tourB = q.ants[i].tourB
				crossTag = 0
			} else {
				crossTag++
			}
		}
		//更新路径矩阵信息
		q.UpdateTourB()
		//更新信息素矩阵
		q.UpdatePheromone()
		//旋转们操作
		q.Mutation()
		if crossTag >= MAX_TAG {
			crossTag = 0
			// 量子交叉
			// 。。。
			break
		}
		//动态更新旋转角
		q.date = q.maxDate - ((q.maxDate-q.minDate)/float64(q.iterNum))*float64(runtimes)
	}
	q.gaMain.time = time.Now().UnixNano() - start
	fmt.Println("\nQAca耗时：", float64(q.gaMain.time)/1000000000)
	// 更新状态
	q.UpdateStatus()
	q.Path()
	// GA优化
	q.GaPathCompute()
	fmt.Printf("\n进行GA的次数为：%d\n", q.gaMain.GaTimes)
}

func (q *qAca) UpdateStatus() {
	q.gaMain.CityPos = q.cityP
	q.gaMain.LenChrom = len(q.cityP)
	q.gaMain.Chrom = make([][]int, SIZE_POP)
	length := len(q.cityP)
	for i := 0; i < SIZE_POP; i++ {
		q.gaMain.Chrom[i] = make([]int, length)
	}
	q.gaMain.BestResult = make([]int, length)
	q.gaMain.Path = q.bestTour
}

func (q *qAca) GaPathCompute() []int {
	return q.gaMain.Run()
}

func (q *qAca) Path() {
	p := q.gaMain.Path
	fmt.Print("x12 = [")
	for i := 0; i < len(p); i++ {
		if i == len(p)-1 {
			break
		}
		fmt.Print(q.gaMain.CityPos[p[i]][0], ",")
	}
	fmt.Println(q.gaMain.CityPos[p[len(p)-1]][0], "];")
	fmt.Print("y12 = [")
	for i := 0; i < len(p); i++ {
		if i == len(p)-1 {
			break
		}
		fmt.Print(q.gaMain.CityPos[p[i]][1], ",")
	}
	fmt.Println(q.gaMain.CityPos[p[len(p)-1]][1], "];")
}

func (q *qAca) UpdatePheromone() {
	//信息素挥发
	for i := 0; i < q.cityNum; i++ {
		for j := 0; j < q.cityNum; j++ {
			q.tao[i][j] = (1 - q.p) * q.tao[i][j]
		}
	}
	//信息素更新
	for k := 0; k < q.popNum; k++ {
		for i := 0; i < q.cityNum; i++ {
			for j := 0; j < q.cityNum; j++ {
				ant := q.ants[k]
				q.tao[ant.tour[j]][ant.tour[j+1]] +=
					math.Pow(math.Pow(ant.pop[i][j][1], 2), q.y) * (1 / ant.tourLength)
			}
		}
	}
}

func (q *qAca) UpdateTourB() {
	for i := 0; i < q.popNum; i++ {
		for j := 0; j < q.cityNum; j++ {
			ant := q.ants[i]
			q.ants[i].tourB[ant.tour[j]][ant.tour[j+1]],
				q.ants[i].tourB[ant.tour[j+1]][ant.tour[j]] = 1, 1
		}
	}
}

func (q *qAca) Mutation() {
	cos, sin, fCos, fSin := math.Cos(q.date), math.Sin(q.date), math.Cos(-q.date), math.Sin(-q.date)
	for i := 0; i < q.popNum; i++ {
		pop := q.ants[i].pop
		for j := 0; j < q.cityNum; j++ {
			for k := 0; k < q.cityNum; k++ {
				ret := pop[j][k][0] * pop[j][k][1]
				if ret >= 0 && q.tourB[j][k] == 1 {
					pop[j][k][0] = pop[j][k][0]*cos - pop[j][k][1]*sin
					pop[j][k][1] = pop[j][k][0]*sin + pop[j][k][1]*cos
				} else if ret >= 0 && q.tourB[j][k] == 0 {
					// -
					pop[j][k][0] = pop[j][k][0]*fCos - pop[j][k][1]*fSin
					pop[j][k][1] = pop[j][k][0]*fSin + pop[j][k][1]*fCos
				} else if ret < 0 && q.tourB[j][k] == 1 {
					// -
					pop[j][k][0] = pop[j][k][0]*fCos - pop[j][k][1]*fSin
					pop[j][k][1] = pop[j][k][0]*fSin + pop[j][k][1]*fCos
				} else if ret < 0 && q.tourB[j][k] == 0 {
					// +
					pop[j][k][0] = pop[j][k][0]*cos - pop[j][k][1]*sin
					pop[j][k][1] = pop[j][k][0]*sin + pop[j][k][1]*cos
				}
			}
		}
	}
}

func (q *qAca) CrossBySun(popA, popB *QAnt) [][][2]float64 {
	db := make([][][2]float64, q.cityNum)
	for i := 0; i < q.popNum; i++ {
		db[i] = make([][2]float64, q.cityNum)
		for j := 0; j < q.cityNum; j++ {
			db[i][j][0] = (popA.pop[i][j][0] + popB.pop[i][j][0]) / 2
			db[i][j][1] = math.Sqrt(1 - db[i][j][0]*db[i][j][0])
		}
	}
	return db
}

/**
 * 初始化量子蚁群 QA(t)
 * 为了使算法初始搜索时所有状态均以相同
 * 概率出现，QA(0) 中所有概率幅 αij 、βij 均取为1/ 根号2
 */
func initPop(popNum, cityNum int) []QAnt {
	ants := make([]QAnt, popNum)
	for i := 0; i < popNum; i++ {
		db := make([][][2]float64, cityNum)
		for j := 0; j < cityNum; j++ {
			db[j] = make([][2]float64, cityNum)
			for k := 0; k < cityNum; k++ {
				//概率幅 αij 、βij 均取为 1/ 根号2
				v := 1 / math.Sqrt(2)
				db[j][k][0], db[j][k][1] = v, v
			}
		}
		ants[i] = NewQAnt(cityNum, db)
	}
	return ants
}

// 初始化信息素
func initTao(cityNum int) [][]float64 {
	tao := make([][]float64, cityNum)
	for i := 0; i < cityNum; i++ {
		tao[i] = make([]float64, cityNum)
		for j := 0; j < cityNum; j++ {
			tao[i][j] = 0.1
		}
	}
	return tao
}

//初始化距离矩阵
func initDistance(city [][2]float64, cityNum int) [][]float64 {
	distance := make([][]float64, cityNum)
	for i := 0; i < cityNum; i++ {
		distance[i] = make([]float64, cityNum)
	}
	//计算两个城市之间的距离矩阵，并更新距离矩阵
	for city1 := 0; city1 < cityNum-1; city1++ {
		for city2 := city1 + 1; city2 < cityNum; city2++ {
			distance[city1][city2] = math.Sqrt((city[city1][0]-city[city2][0])*(city[city1][0]-city[city2][0]) +
				(city[city1][1]-city[city2][1])*(city[city1][1]-city[city2][1]))
			//距离矩阵是对称矩阵
			distance[city2][city1] = distance[city1][city2]
		}
	}
	return distance
}
