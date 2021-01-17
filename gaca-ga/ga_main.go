package gaca_ga

import (
	"fmt"
	"math"
	"time"
)

type GaMain struct {
	Path           []int        // 存放原始路径
	LenChrom       int          //染色体长度(这里指城市个数)
	CityPos        [][2]float64 //存放城市的X、Y坐标
	Chrom          [][]int      //种群
	BestResult     []int        // 最佳路线
	MinDistance    float64      //最佳路径长度
	Length         float64      //最优距离长度
	time           int64        // 耗时
	NeedContinueGa bool         // 算法是否需要GA优化
	GaTimes        int          // 当前ga次数
	ps             []line       // 线段
	ga             GA           // GA 实例，可防止需要进行多次ga优化时进行多次重新创建
}

func NewGaMain() *GaMain {
	lenChrom := 16
	return &GaMain{
		Path:           nil,
		LenChrom:       lenChrom,
		CityPos:        nil,
		Chrom:          make([][]int, SIZE_POP),
		BestResult:     make([]int, lenChrom),
		MinDistance:    0.0,
		Length:         0,
		time:           0,
		NeedContinueGa: true,
		GaTimes:        0,
		ga:             nil,
	}
}

// 重置变量
func (g *GaMain) reset() {
	g.Length = math.MaxFloat64
	g.time = 0
	g.MinDistance = math.MaxFloat64
}

// 运行 ga算法
func (g *GaMain) Run() []int {
	return g.run(true)
}

// 运行 ga算法
// rag 为 true 表示 GA算法采用具有先验知识的初始化
func (g *GaMain) run(tag bool) []int {
	if g.ga == nil {
		g.ga = newGA(g)
	} else {
		g.reset()
	}
	start := time.Now().UnixNano() // 开始计时
	// 初始化种群
	g.ga.Init(tag)
	//最短路径出现代数
	bestFitIndex := 0
	distanceArr := make([]float64, SIZE_POP)
	var dis float64
	for j := 0; j < SIZE_POP; j++ {
		dis = g.ga.PathLen(g.Chrom[j])
		distanceArr[j] = dis
	}
	// 计算最短路径及序号
	bestIndex := g.ga.Min(&distanceArr)
	// 最短距离
	g.MinDistance = bestIndex[1]
	// 最短序号
	index := int(bestIndex[0])
	for j := 0; j < g.LenChrom; j++ {
		// 最短路径序列
		g.BestResult[j] = g.Chrom[index][j]
	}
	// 开始进化
	var newArr []float64
	var newMinDis float64
	var newIndex int
	for i := 0; i < MAXGEN; i++ {
		// 计算
		g.compute()
		// 距离数组
		for j := 0; j < SIZE_POP; j++ {
			distanceArr[j] = g.ga.PathLen(g.Chrom[j])
		}
		newArr = g.ga.Min(&distanceArr)
		// 新的最短路径
		newMinDis = newArr[1]
		if newMinDis < g.MinDistance {
			// 更新最短路径
			g.MinDistance = newMinDis
			newIndex = int(newArr[0])
			for j := 0; j < g.LenChrom; j++ {
				// 更新最短路径序列
				g.BestResult[j] = g.Chrom[newIndex][j]
			}
			// 最短路径索引
			bestFitIndex = i + 1
		}
	}
	// 是否需要继续进行GA，就是检查是否有交叉的
	g.NeedContinueGa = g.checkIntersectLine()
	if g.GaTimes >= MAX_GA_TIMES {
		g.NeedContinueGa = false
	} else {
		g.GaTimes++
	}
	path := make([]int, g.LenChrom+1)
	tap := 0
	j := 0
	for i := 0; i < g.LenChrom; i++ {
		if g.BestResult[i] == 1 {
			tap = i
			break
		}
	}
	// 从第0个开始，把后面的逐个添加
	for i := tap; i < g.LenChrom; i++ {
		path[j] = g.BestResult[i] - 1
		j++
	}
	// 然后从下标==0开始到第0个，继续逐个添加
	for i := 0; i < tap; i++ {
		path[j] = g.BestResult[i] - 1
		j++
	}
	// 计算结束
	finish := time.Now().UnixNano()
	g.time += finish - start
	fmt.Println("Ga耗时", float64(finish-start)/1000000000)
	path[j] = g.BestResult[tap] - 1
	g.Length = g.MinDistance
	g.Path = path

	// 打印数据
	g.print(tap, bestFitIndex)

	// 如果需要继续优化，就继续
	if g.NeedContinueGa {
		g.run(true)
	}
	return g.Path
}

// 检查线段集是否有相交的
// city 点集合 double[][2]
// 点连接的顺序
// return true：有，false：无 ， 要是输入数据有错误，也是会返回false
func (g *GaMain) checkIntersectLine() bool {
	return g.checkIntersectLine2()
}

// 检查线段集是否有相交的
func (g *GaMain) checkIntersectLine2() bool {
	g.wrapper()
	length := len(g.ps)
	if length <= 3 {
		return false
	}
	for i := 0; i < length-2; i++ {
		for j := i + 2; j < length; j++ {
			// 要排除前后两个，前面的可以不考虑了
			if i == 0 && j == length-1 {
				continue
			}
			if isIntersectLine(g.ps[i].a, g.ps[i].b, g.ps[j].a, g.ps[j].b) {
				// 存在线段相交
				return true
			}
		}
	}
	// 不存在线段相交
	return false
}

/**
 * 判断两条线段是否相交
 *
 * 至此，判断2条线段是否相交的思路以及很清晰了。 首先，我们可以简单的用一下边界条件，
 * 如果某线段的横(纵)坐标的最小值大于另一条线段的横纵坐标的最大值，或反之，则肯定不相交，
 * 如果满足上述条件，则使用上述方法计算叉积，若2个叉积结果大于0，则相交，否则，不想交。
 * 这样代码就so easy了，就不用我写出来啦~
 * 参考：https://blog.csdn.net/s0rose/article/details/78831570?utm_medium=distribute.pc_relevant.none-task-blog-BlogCommendFromMachineLearnPai2-4.channel_param&depth_1-utm_source=distribute.pc_relevant.none-task-blog-BlogCommendFromMachineLearnPai2-4.channel_param
 * @return true：相交，false：不相交
 */
func isIntersectLine(x1, x2, x3, x4 point) bool {
	// 快速排斥 以 l1 l2对角线的矩形必相交，否则不相交
	if math.Max(x1.x, x2.x) >= math.Min(x3.x, x4.x) && // 矩形1最右端大于矩形2最左端
		math.Max(x3.x, x4.x) >= math.Min(x1.x, x2.x) && // 矩形2最右端大于矩形最左端
		math.Max(x1.y, x2.y) >= math.Min(x3.y, x4.y) && // 矩形1最高端大于矩形最低端
		math.Max(x3.y, x4.y) >= math.Min(x1.y, x2.y) { //矩形2最高端大于矩形最低端
		// 若通过快速排斥则进行跨立实验
		if cross(x1, x2, x3)*cross(x1, x2, x4) <= 0 &&
			cross(x3, x4, x1)*cross(x3, x4, x2) <= 0 {
			return true
		}
	}
	return false
}

// 跨立实验
func cross(x1, x2, x3 point) float64 {
	a := x2.x - x1.x
	b := x2.y - x1.y
	c := x3.x - x1.x
	d := x3.y - x1.y
	return a*d - c*b
}

// 转为 线段 集合
func (g *GaMain) wrapper() {
	ret := make([]line, 0)
	length := len(g.Path) - 1
	for i := 0; i < length; i++ {
		ret = append(ret, line{
			a: point{
				x: g.CityPos[g.Path[i]][0],
				y: g.CityPos[g.Path[i]][1],
			},
			b: point{
				x: g.CityPos[g.Path[i+1]][0],
				y: g.CityPos[g.Path[i+1]][1],
			},
		})
	}
	g.ps = ret
}
func (g *GaMain) compute() {
	g.ga.Choice()
	g.ga.Cross()
	g.ga.Mutation()
	g.ga.Reverse()
}
func (g *GaMain) print(tag, bestFitIndex int) {
	if !OPEN_PRINT_KEY {
		return
	}
	fmt.Println("****************************************")
	fmt.Printf("本程序使用遗传算法求解规模为%d的TSP问题,种群数目为:%d,进化代数为:%d\n", g.LenChrom, SIZE_POP, MAXGEN)
	fmt.Print("得到最短路径为:")
	for i := 0; i < g.LenChrom-1; i++ {
		fmt.Print(g.BestResult[i]-1, "->")
	}
	fmt.Println(g.BestResult[g.LenChrom-1] - 1)
	fmt.Println("改变后的输出最短路径为：", g.BestResult[tag]-1)
	fmt.Printf("该次遗传算法-最短路径长度为:%f,得到最短路径在第%d代.\n", g.MinDistance, bestFitIndex)
	fmt.Printf("该次遗传算法程序耗时:%d秒.\n", g.time)
	fmt.Printf("===>>>path = %v", g.Path)
	fmt.Printf("是否存在相交：%v\n", g.NeedContinueGa)
	fmt.Println("****************************************")
}

type point struct {
	x, y float64
}
type line struct {
	a, b point
}
