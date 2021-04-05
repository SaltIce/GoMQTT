package logger

import (
	"Go-MQTT/Logger"
	"Go-MQTT/mqtt_v5/config"
	"bytes"
	"fmt"
	"github.com/buguang01/util"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type severity int32

var timeNow = time.Now

// buffer holds a byte Buffer for reuse. The zero value is ready for use.
type buffer struct {
	bytes.Buffer
	tmp  [64]byte // temporary byte array for creating headers.
	next *buffer
}

var (
	InfoOpen  = true
	level     = Logger.LogModeFmt // #LogMode = 0 //默认   LogMode = 1 //只输出main文件    LogMode = 2 //只出输cmd
	logPath   = "logs"
	DebugOpen = true
)

const (
	infoLog severity = iota
	warningLog
	errorLog
	fatalLog
	numSeverity = 4
)

const severityChar = "IWEF"

var severityName = []string{
	infoLog:    "INFO",
	warningLog: "WARNING",
	errorLog:   "ERROR",
	fatalLog:   "FATAL",
}

func init() {
	consts := config.ConstConf.Logger
	InfoOpen = consts.InfoOpen
	DebugOpen = consts.DebugOpen
	util.SetLocation(util.BeiJing)
	if !InfoOpen {
		fmt.Println("日志系统 未开启 info日志打印记录")
	}
	if !DebugOpen {
		fmt.Println("日志系统 未开启 debug日志打印记录")
	}
	if consts.Level > uint(Logger.LogModeFmt) {
		panic(fmt.Sprintf("level %v unSupport\n", consts.Level))
	}
	level = Logger.LogMode(consts.Level)
	if consts.LogPath != "" {
		logPath = consts.LogPath
	}
	init2()
}

//添加一个封装log，方便以后修改log框架
//正常退出日志模块
func init2() {
	//关闭日志服务
	//defer Logger.LogClose()

	defer func() {
		r := recover()
		if r != nil {
			Logger.PFatal(r)
		}
	}()
	//初始化日志模块  这里暂时只输出到cmd中，Logger.LogModeMain可以输出到文件中去的
	minlv := Logger.LogLevel(0)
	Logger.Init(minlv, logPath, level)
	Logger.PInfo(fmt.Sprintf("初始化日志模块==>> minlv：%d，pathstr：%s，logmode：%d", 0, logPath, level))
	//设置监听keyid ,这个keyid的日志会单独再输出到一个文件中
	id := 1001
	Logger.SetListenKeyID(id)
	Logger.PInfo("设置监听keyid : " + strconv.Itoa(id) + " ,这个keyid的日志会单独再输出到一个文件中")
	Logger.PInfo("info日志初始化")
	Logger.PInfoKey("infoKey日志初始化", id)
	Logger.PDebug("debug日志初始化")
	Logger.PDebugKey("debugKey日志初始化", id)
	//Logger.PStatus("status日志初始化", 100)
	//Logger.PStatusKey("statusKey日志初始化", 100,123)
	//glog.Error("日志服务启动成功")
	//errs := fmt.Errorf(string(debug.Stack()))
}

/**
 info级别
**/
func Info(msg string) {
	if !InfoOpen {
		return
	}
	//这里两次字符串拼接 用 + 拼接速度快
	Logger.PInfo(getfile() + msg)
}

func Infof(format string, args ...interface{}) {
	if !InfoOpen {
		return
	}
	Logger.PInfo(getfile() + getStringFormat(format, args...))
}

/**
 error级别
**/
func Error(err error, msg string) {
	Logger.PError(err, getfile()+msg)
}
func Errorf(err error, msg string, args ...interface{}) {
	Logger.PError(err, getfile()+getStringFormat(msg, args...))
}
func ErrorNoStackInfof(msg string, args ...interface{}) {
	Logger.PErrorNoStackInfo(getfile() + getStringFormat(msg, args...))
}

/**
 debug级别
**/
func Debug(msg string) {
	if !DebugOpen {
		return
	}
	Logger.PDebug(getfile() + msg)
}
func Debugf(format string, args ...interface{}) {
	if !DebugOpen {
		return
	}
	Logger.PDebug(getfile() + getStringFormat(format, args...))
}

/**
 status级别
**/
func Status(msg string) {
	Logger.PStatus(getfile() + msg)
}
func Statusf(format string, args ...interface{}) {
	Logger.PStatus(getfile() + getStringFormat(format, args...))
}

//将格式化字符串而不打印
func getStringFormat(format string, args ...interface{}) string {
	if format != "" {
		format = fmt.Sprintf(format, args...)
	}
	return format
}

func getfile() string {
	var method string
	//深度为2，表示调用Logger里面方法的那个人所在的地方
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 1
	} else {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			file = file[slash+1:]
		}

		me := runtime.FuncForPC(pc)
		if me == nil {
			method = "unnamed"
		} else {
			method = me.Name()
			dot := strings.LastIndex(method, ".")
			if dot >= 0 {
				method = method[dot+1:]
			}
		}
	}
	/**
		1. + 连接适用于短小的、常量字符串（明确的，非变量），因为编译器会给我们优化。
		2. Join是比较统一的拼接，不太灵活
		3. fmt和buffer基本上不推荐
		4. builder从性能和灵活性上，都是上佳的选择
	**/
	/**
		Join和Builder。这两个方法的使用侧重点有些不一样，
		如果有现成的数组、切片那么可以直接使用Join,
		但是如果没有，并且追求灵活性拼接，还是选择Builder。
		Join还是定位于有现成切片、数组的（毕竟拼接成数组也要时间），
		并且使用固定方式进行分解的，比如逗号、空格等，局限比较大
	**/
	builder := strings.Builder{}
	builder.WriteString("【file=")
	builder.WriteString(file)
	builder.WriteString("；line=")
	builder.WriteString(strconv.Itoa(line))
	builder.WriteString("；method=")
	builder.WriteString(method)
	builder.WriteString("】 ")
	return builder.String()
}
