package log

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"math"
	"os"
	"path/filepath"
)

var Logger *logrus.Logger
var LogLevelList map[string]logrus.Level = map[string]logrus.Level{
	"panic": logrus.PanicLevel,
	"fatal": logrus.FatalLevel,
	"error": logrus.ErrorLevel,
	"warn":  logrus.WarnLevel,
	"info":  logrus.InfoLevel,
	"debug": logrus.DebugLevel,
	"trace": logrus.TraceLevel,
}

func LoggerInit(level string) {
	Logger = logrus.New()
	appPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		Logger.Panicf("服务执行路径获取失败：%s", err.Error())
	}
	hook := lumberjack.Logger{
		Filename:   appPath + "/log/log.log",      // 日志文件路径
		MaxSize:    int(math.Max(float64(10), 1)), // 每个日志文件保存的最大尺寸 单位：M
		MaxBackups: int(math.Max(float64(5), 1)),  // 日志文件最多保存多少个备份
		MaxAge:     int(math.Max(float64(30), 1)), // 文件最多保存多少天
		Compress:   true,                          // 是否压缩
	}
	Logger.SetOutput(io.MultiWriter(&hook, os.Stdout))
	logLevel, ok := LogLevelList[level]
	if !ok {
		Logger.Fatalf("日志log_level“%s”不存在,请检查config.json配置", level)
	}
	Logger.SetLevel(logLevel)
	Logger.SetFormatter(
		&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "datetime",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "caller",
			},
		},
	)
}

func GinLogFormatter(params gin.LogFormatterParams) string {
	Logger.Infof("client_ip: %s |Method: %s |Path: %s |StatusCode: %d|Latency: %s|",
		params.ClientIP,
		params.Method,
		params.Path,
		params.StatusCode,
		params.Latency,
	)
	return ""
}
