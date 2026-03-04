package logger

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// Logger 定义日志接口
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

// logrusLogger 封装 logrus.Logger 以实现 Logger 接口
type logrusLogger struct {
	*logrus.Logger
}

// 全局日志实例
var global Logger = &logrusLogger{Logger: logrus.New()}

// InitLogger 使用适当的配置初始化全局日志记录器
func InitLogger(debug bool) {
	logger := logrus.New()

	// 设置日志格式
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006/01/02 15:04:05",
	})

	// 设置输出
	logger.SetOutput(os.Stdout)

	// 设置日志级别
	if debug {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	global = &logrusLogger{Logger: logger}
}

// SetOutput 设置日志记录器的输出目标
func SetOutput(w io.Writer) {
	if l, ok := global.(*logrusLogger); ok {
		l.Logger.SetOutput(w)
	}
}

// SetLevel 设置日志级别
func SetLevel(level logrus.Level) {
	if l, ok := global.(*logrusLogger); ok {
		l.Logger.SetLevel(level)
	}
}

// Debug 记录调试信息
func Debug(args ...interface{}) {
	global.Debug(args...)
}

// Debugf 记录格式化的调试信息
func Debugf(format string, args ...interface{}) {
	global.Debugf(format, args...)
}

// Info 记录普通信息
func Info(args ...interface{}) {
	global.Info(args...)
}

// Infof 记录格式化的普通信息
func Infof(format string, args ...interface{}) {
	global.Infof(format, args...)
}

// Warning 记录警告信息
func Warning(args ...interface{}) {
	global.Warning(args...)
}

// Warningf 记录格式化的警告信息
func Warningf(format string, args ...interface{}) {
	global.Warningf(format, args...)
}

// Error 记录错误信息
func Error(args ...interface{}) {
	global.Error(args...)
}

// Errorf 记录格式化的错误信息
func Errorf(format string, args ...interface{}) {
	global.Errorf(format, args...)
}

// Fatal 记录致命错误信息并退出程序
func Fatal(args ...interface{}) {
	global.Fatal(args...)
}

// Fatalf 记录格式化的致命错误信息并退出程序
func Fatalf(format string, args ...interface{}) {
	global.Fatalf(format, args...)
}
