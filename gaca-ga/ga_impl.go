package gaca_ga

import (
	"math"
	"math/rand"
	"strings"
	"time"
)

type ga struct {
	gaMain        *GaMain
	cacheDistinct map[string]float64
	r             *rand.Rand
}

func newGA(gaMain *GaMain) GA {
	//return &gaWrapper{g: &ga{gaMain: gaMain, cacheDistinct: map[string]float64{}}}
	return &ga{gaMain: gaMain, cacheDistinct: map[string]float64{}, r: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// 种群初始化函数
func (g *ga) Init(tag bool) {
	num := SIZE_POP / 5
	for num < SIZE_POP {
		if tag {
			for i := 0; i < SIZE_POP/4; i++ {
				for j := 0; j < g.gaMain.LenChrom; j++ {
					//将蚁群算法得到的局部最优作为初始种群，进行遗传算法加速计算
					g.gaMain.Chrom[i][j] = g.gaMain.Path[j] + 1
				}
			}
			for i := SIZE_POP / 4; i < SIZE_POP; i++ {
				for j := 0; j < g.gaMain.LenChrom; j++ {
					g.gaMain.Chrom[i][j] = j + 1
				}
			}
		} else {
			for i := 0; i < SIZE_POP; i++ {
				for j := 0; j < g.gaMain.LenChrom; j++ {
					g.gaMain.Chrom[i][j] = j + 1
				}
			}
		}
		num++
		for i := 0; i < g.gaMain.LenChrom-1; i++ {
			for j := i + 1; j < g.gaMain.LenChrom; j++ {
				temp := g.gaMain.Chrom[num][i]
				g.gaMain.Chrom[num][i] = g.gaMain.Chrom[num][j]
				// 交换第num个个体的第i个元素和第j个元素
				g.gaMain.Chrom[num][j] = temp
				num++
				if num >= SIZE_POP {
					break
				}
			}
			if num >= SIZE_POP {
				break
			}
		}
		// 如果经过上面的循环还是无法产生足够的初始个体，则随机再补充一部分
		// 具体方式就是选择两个基因位置，然后交换
		for num < SIZE_POP {
			r1 := g.r.Float64()
			r2 := g.r.Float64()
			// 位置1
			p1 := int(float64(g.gaMain.LenChrom) * r1)
			// 位置2
			p2 := int(float64(g.gaMain.LenChrom) * r2)
			temp := g.gaMain.Chrom[num][p1]
			g.gaMain.Chrom[num][p1] = g.gaMain.Chrom[num][p2]
			// 交换基因位置
			g.gaMain.Chrom[num][p2] = temp
			num++
		}
	}
}

// 计算两个城市之间的距离
func (g *ga) Distance(aCity, bCity [2]float64) float64 {
	x1 := aCity[0]
	y1 := aCity[1]
	x2 := bCity[0]
	y2 := bCity[1]
	// 下面这个会非常耗时，因为会有大量的调用，性能很差
	//key := strconv.FormatFloat(x1, 'f', -1, 64) +
	//	strconv.FormatFloat(x2, 'f', -1, 64) +
	//	strconv.FormatFloat(y1, 'f', -1, 64) +
	//	strconv.FormatFloat(y2, 'f', -1, 64)
	//if cacheD, ok := g.cacheDistinct[key]; ok {
	//	return cacheD
	//}
	v := math.Sqrt((x1-x2)*(x1-x2) + (y1-y2)*(y1-y2))
	//g.cacheDistinct[key] = v
	return v
}

// 计算距离数组的最小值
func (g *ga) Min(min1 *[]float64) []float64 {
	bestIndex := make([]float64, 2)
	min := *min1
	minDis := min[0]
	minIndex := 0
	for i := 1; i < SIZE_POP; i++ {
		dis := min[i]
		if dis < minDis {
			minDis = dis
			minIndex = i
		}
	}
	bestIndex[0] = float64(minIndex)
	bestIndex[1] = minDis
	return bestIndex
}

// 计算某一个方案的路径长度，适应度函数为路线长度的倒数
func (g *ga) PathLen(bestResult []int) float64 {
	// 初始化路径长度
	var path float64 = 0
	for i := 0; i < g.gaMain.LenChrom-1; i++ {
		index1 := bestResult[i]
		index2 := bestResult[i+1]
		dis := g.Distance(g.gaMain.CityPos[index1-1], g.gaMain.CityPos[index2-1])
		path += dis
	}
	// 最后一个城市序号
	lastIndex := bestResult[g.gaMain.LenChrom-1]
	// 第一个城市序号
	firstIndex := bestResult[0]
	lastDis := g.Distance(g.gaMain.CityPos[lastIndex-1], g.gaMain.CityPos[firstIndex-1])
	path = path + lastDis
	// 返回总的路径长度
	return path
}

func (g *ga) Choice() {
	var pick float64 = 0
	choiceArr := make([][]int, SIZE_POP)
	fitPro := make([]float64, SIZE_POP)
	var sum float64 = 0
	// 适应度函数数组(距离的倒数)
	fit := make([]float64, SIZE_POP)
	for j := 0; j < SIZE_POP; j++ {
		path := g.PathLen(g.gaMain.Chrom[j])
		fitness := 1 / path
		fit[j] = fitness
		sum += fitness
	}
	for j := 0; j < SIZE_POP; j++ {
		// 概率数组
		fitPro[j] = fit[j] / sum
	}
	// 开始轮盘赌
	for i := 0; i < SIZE_POP; i++ {
		// 0到1之间的随机数
		pick = g.r.Float64()
		choiceArr[i] = make([]int, g.gaMain.LenChrom)
		for j := 0; j < SIZE_POP; j++ {
			pick = pick - fitPro[j]
			if pick <= 0 {
				for k := 0; k < g.gaMain.LenChrom; k++ {
					// 选中一个个体
					choiceArr[i][k] = g.gaMain.Chrom[j][k]
				}
				break
			}
		}
	}
	for i := 0; i < SIZE_POP; i++ {
		for j := 0; j < g.gaMain.LenChrom; j++ {
			g.gaMain.Chrom[i][j] = choiceArr[i][j]
		}
	}
}

func (g *ga) Cross() {
	pick, pick1, pick2 := float64(0), float64(0), float64(0)
	choice1, choice2, pos1, pos2, temp := 0, 0, 0, 0, 0
	// 冲突位置
	conflict1, conflict2 := make([]int, g.gaMain.LenChrom), make([]int, g.gaMain.LenChrom)
	// move 当前移动的位置
	num1, num2, index1, index2, move := 0, 0, 0, 0, 0
	for move < g.gaMain.LenChrom-1 {
		// 用于决定是否进行交叉操作
		pick = g.r.Float64()
		if pick > PCROSS {
			move += 2
			continue // 本次不进行交叉
		}
		// 采用部分映射杂交
		// 用于选取杂交的两个父代
		choice1 = move
		// 注意避免下标越界
		choice2 = move + 1
		//如果两个父类相似度大于50% ，本次不进行交叉
		//            if(strSimilar(Arrays.toString(gaMain.getChrom()[choice1]), Arrays.toString(gaMain.getChrom()[choice2]))>0.9){
		//                System.err.println("本次不进行交叉");
		//                continue; // 本次不进行交叉
		//            }
		pick1 = g.r.Float64()
		pick2 = g.r.Float64()
		// 用于确定两个杂交点的位置
		pos1 = int(pick1 * float64(g.gaMain.LenChrom))
		pos2 = int(pick2 * float64(g.gaMain.LenChrom))
		for pos1 > g.gaMain.LenChrom-2 || pos1 < 1 {
			pick1 = g.r.Float64()
			pos1 = int(pick1 * float64(g.gaMain.LenChrom))
		}
		for pos2 > g.gaMain.LenChrom-2 || pos2 < 1 {
			pick2 = g.r.Float64()
			pos2 = int(pick2 * float64(g.gaMain.LenChrom))
		}
		if pos1 > pos2 {
			// 交换pos1和pos2的位置
			pos1 ^= pos2
			pos2 ^= pos1
			pos1 ^= pos2
		}
		for j := pos1; j <= pos2; j++ {
			temp = g.gaMain.Chrom[choice1][j]
			g.gaMain.Chrom[choice1][j] = g.gaMain.Chrom[choice2][j]
			// 逐个交换顺序
			g.gaMain.Chrom[choice2][j] = temp
		}
		num1 = 0
		num2 = 0

		if pos1 > 0 && pos2 < g.gaMain.LenChrom-1 {
			for j := 0; j <= pos1-1; j++ {
				for k := pos1; k <= pos2; k++ {
					if g.gaMain.Chrom[choice1][j] == g.gaMain.Chrom[choice1][k] {
						conflict1[num1] = j
						num1++
					}
					if g.gaMain.Chrom[choice2][j] == g.gaMain.Chrom[choice2][k] {
						conflict2[num2] = j
						num2++
					}
				}
			}
			for j := pos2 + 1; j < g.gaMain.LenChrom; j++ {
				for k := pos1; k <= pos2; k++ {
					if g.gaMain.Chrom[choice1][j] == g.gaMain.Chrom[choice1][k] {
						conflict1[num1] = j
						num1++
					}
					if g.gaMain.Chrom[choice2][j] == g.gaMain.Chrom[choice2][k] {
						conflict2[num2] = j
						num2++
					}
				}
			}
		}
		if (num1 == num2) && num1 > 0 {
			for j := 0; j < num1; j++ {
				index1 = conflict1[j]
				index2 = conflict2[j]
				// 交换冲突的位置
				temp = g.gaMain.Chrom[choice1][index1]
				g.gaMain.Chrom[choice1][index1] = g.gaMain.Chrom[choice2][index2]
				g.gaMain.Chrom[choice2][index2] = temp
			}
		}
		move += 2
	}
}

func (g *ga) Mutation() {
	pick, pick1, pick2 := float64(0), float64(0), float64(0)
	pos1, pos2, temp := 0, 0, 0
	for i := 0; i < SIZE_POP; i++ {
		// 用于判断是否进行变异操作
		pick = g.r.Float64()
		if pick > PMUTATION {
			continue
		}
		pick1 = g.r.Float64()
		pick2 = g.r.Float64()
		// 选取进行变异的位置
		pos1 = int(pick1 * float64(g.gaMain.LenChrom))
		pos2 = int(pick2 * float64(g.gaMain.LenChrom))
		for pos1 > g.gaMain.LenChrom-1 {
			pick1 = g.r.Float64()
			pos1 = int(pick1 * float64(g.gaMain.LenChrom))
		}
		for pos2 > g.gaMain.LenChrom-1 {
			pick2 = g.r.Float64()
			pos2 = int(pick2 * float64(g.gaMain.LenChrom))
		}
		temp = g.gaMain.Chrom[i][pos1]
		g.gaMain.Chrom[i][pos1] = g.gaMain.Chrom[i][pos2]
		g.gaMain.Chrom[i][pos2] = temp
	}
}

func (g *ga) Reverse() {
	pick1, pick2, dis, reverseDis := float64(0), float64(0), float64(0), float64(0)
	n, flag, pos1, pos2 := 0, 0, 0, 0
	reverseArr := make([]int, g.gaMain.LenChrom)
	for i := 0; i < SIZE_POP; i++ {
		// 用于控制本次逆转是否有效
		flag = 0
		for flag == 0 {
			pick1 = g.r.Float64()
			pick2 = g.r.Float64()
			//选取进行逆转操作的位置
			pos1 = int(pick1 * float64(g.gaMain.LenChrom))
			pos2 = int(pick2 * float64(g.gaMain.LenChrom))
			for pos1 > g.gaMain.LenChrom-1 {
				pick1 = g.r.Float64()
				pos1 = int(pick1 * float64(g.gaMain.LenChrom))
			}
			for pos2 > g.gaMain.LenChrom-1 {
				pick2 = g.r.Float64()
				pos2 = int(pick2 * float64(g.gaMain.LenChrom))
			}
			if pos1 > pos2 {
				// 交换使得pos1 <= pos2
				pos1 ^= pos2
				pos2 ^= pos1
				pos1 ^= pos2
			}
			// 不用管等于的
			if pos1 < pos2 {
				for j := 0; j < g.gaMain.LenChrom; j++ {
					// 复制数组
					reverseArr[j] = g.gaMain.Chrom[i][j]
				}
				// 记录逆转数目
				n = 0
				for j := pos1; j <= pos2; j++ {
					// 逆转数组
					reverseArr[j] = g.gaMain.Chrom[i][pos2-n]
					n++
				}
				// 逆转之后的距离
				reverseDis = g.PathLen(reverseArr)
				// 原始距离
				dis = g.PathLen(g.gaMain.Chrom[i])
				if reverseDis < dis {
					for j := 0; j < g.gaMain.LenChrom; j++ {
						// 更新个体
						g.gaMain.Chrom[i][j] = reverseArr[j]
					}
				}
			}
			flag = 1
		}
	}

}

func (g *ga) StrSimilar(str1, str2 string) float64 {
	index := 0
	s1 := strings.Split(str1, "")
	s2 := strings.Split(str2, "")
	for i := 0; i < len(str1); i++ {
		if s1[i] == s2[i] {
			index++
		}
	}
	return float64(index/len(str1) + index%len(str1))
}
