package gaca_ga

import (
	"math"
	"math/rand"
	"time"
)

// 量子蚁群算法
type QAca interface {
	Run()                                       //启动运行
	UpdateStatus()                              //更新状态数据,供 GA 算法使用
	GaPathCompute() []int                       //通过 遗传算法 优化路径
	UpdatePheromone()                           //更新信息素,ant 当前蚂蚁量子个体,tao 全局信息素
	UpdateTourB()                               //更新路径矩阵信息
	Mutation()                                  //量子旋转门操作量子个体
	CrossBySun(popA, popB *QAnt) [][][2]float64 //量子杂交操作
	Path()                                      // 打印规划的路径
}

// 来自蚂蚁
type QAnt struct {
	tag           int            //0代表当前蚂蚁没有被淘汰，1 淘汰
	p0            float64        //指蚂蚁选择当前可能的最优移动方式的概 率，这种最优的移动方式是根据信息素的积累量和启发式 信息值求出的
	pp            float64        //自启发量权重
	a, b, city    int            // city:城市个数
	tour          []int          //蚂蚁获得的路径
	tourB         [][]int        //路径矩阵 选的路径为1，没选为0
	unvisitedCity []int          //取值是0或1，1表示没有访问过，0表示访问过
	tourLength    float64        //蚂蚁获得的路径长度
	pop           [][][2]float64 //量子个体
	r             *rand.Rand     //随机数生成器
}

func NewQAnt(cityCount int, pop [][][2]float64) QAnt {
	tb := make([][]int, cityCount)
	unvisitedCity := make([]int, cityCount)
	t := make([]int, cityCount+1)
	t[0] = 0
	unvisitedCity[0] = 0
	tb[0] = make([]int, cityCount)
	for i := 1; i < cityCount; i++ {
		tb[i] = make([]int, cityCount)
		unvisitedCity[i] = 1
		t[i] = -1
	}
	return QAnt{
		tag:           0,
		p0:            0.6,
		pp:            2,
		a:             1,
		b:             5,
		city:          cityCount,
		tour:          t,
		tourB:         tb,
		unvisitedCity: unvisitedCity,
		tourLength:    0,
		pop:           pop,
		r:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

/**
 * 随机分配蚂蚁到某个城市中 ，如果
 * 同时完成蚂蚁包含字段的初始化工作
 *  cityCount 总的城市数量
 *  city 蚂蚁分配到那个城市 -1代表随机
 */
func (qa *QAnt) RandomSelectCity(cityCount, city int) {
	qa.unvisitedCity = make([]int, cityCount)
	qa.tourLength = 0
	for i := 0; i < cityCount; i++ {
		qa.tour[i] = -1
		qa.unvisitedCity[i] = 1
	}
	for i := 0; i < cityCount; i++ {
		for j := 0; j < cityCount; j++ {
			qa.tourB[i][j] = 0
		}
	}
	firstCity := 0
	if city == -1 {
		//随机指定第一个城市
		firstCity = qa.r.Intn(cityCount)
	} else {
		//默认从第一个城市开始
		firstCity = city
	}
	//0表示访问过
	qa.unvisitedCity[firstCity] = 0
	//起始城市
	qa.tour[0] = firstCity
}

/**
 * 选择下一个城市
 *  index 需要选择第index个城市了
 *  tao   全局的信息素信息
 *  distance  全局的距离矩阵信息
 */
func (qa *QAnt) SelectNextCity(index int, tao1, distance1 *[][]float64) {
	tao := *tao1
	distance := *distance1

	//蚂蚁所处当前城市
	currentCity := qa.tour[index-1]
	//要选的城市
	selectCity := 0
	var aa float64 = -65526.0
	if qa.r.Float64() <= qa.p0 {
		for i := 0; i < qa.city; i++ {
			//被选过就跳过
			if qa.unvisitedCity[i] == 0 {
				continue
			}
			var b float64 = tao[currentCity][i] * math.Pow(distance[currentCity][i], -qa.pp)
			if b > aa {
				aa = b
				selectCity = i
			}
		}
	} else {
		sum := 0.0 // 算法分母的值
		add := 0.0 // 总的求和值
		//首先用来保存计算的每一个单独的值，然后保存 作为下一步要走的城市的选中概率
		pig := make([]float64, qa.city)
		// 计算分母求和的值
		for i := 0; i < qa.city; i++ {
			//下面这个if是防止出现 除数为0的情况
			if i == currentCity {
				continue
			}
			pig[i] = math.Pow(tao[currentCity][i], float64(qa.a)) * math.Pow(distance[currentCity][i], -float64(qa.b))
			sum += pig[i]
		}
		// 计算总的值
		for i := 0; i < qa.city; i++ {
			if qa.unvisitedCity[i] == 0 {
				pig[i] = 0.0
			} else {
				pig[i] = pig[i] / sum
				add += pig[i]
			}
		}
		selectP := qa.r.Float64() * add
		//轮盘赌选择一个城市；
		sumSelect := 0.0
		//城市选择随机，直到n个概率加起来大于随机数，则选择该城市
		//每次都是顺序走。。。。。
		for i := 0; i < len(pig); i++ {
			sumSelect += pig[i]
			if sumSelect >= selectP {
				selectCity = i
				break
			}
		}
	}
	qa.tour[index] = selectCity
	qa.unvisitedCity[selectCity] = 0
}

/**
 * 计算蚂蚁获得的路径的长度
 * @param distance  全局的距离矩阵信息
 */
func (qa *QAnt) CalTourLength(distance1 *[][]float64) {
	distance := *distance1
	qa.tourLength = 0
	//第一个城市等于最后一个要到达的城市
	qa.tour[qa.city] = qa.tour[0]
	for i := 0; i < qa.city; i++ {
		//从A经过每个城市仅一次，最后回到A的总长度。
		qa.tourLength += distance[qa.tour[i]][qa.tour[i+1]]
	}
}
