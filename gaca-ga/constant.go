package gaca_ga

const (
	MAXGEN int = 50 // 最大进化代数
	// Ga算法参数
	PCROSS    float64 = 0.6 // 交叉概率
	PMUTATION float64 = 0.1 // 变异概率

	OPEN_PRINT_KEY = false // 打印开关
)

var (
	SIZE_POP     int = 50  // ga种群数目
	POP_NUM      int = 50  // 量子蚁群种群数目， 不宜太多
	ITER_NUM     int = 250 // 量子蚁群迭代数目
	MAX_GA_TIMES int = 50  // 最大可以进行的 GA 次数
)
