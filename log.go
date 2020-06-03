package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

func DEBUG_LOG(fmt string, args ...interface{}) { logImp(1, 2, "[DEBUG]", fmt, args) }
func INFO_LOG(fmt string, args ...interface{})  { logImp(2, 2, "[INFO]", fmt, args) }
func ERROR_LOG(fmt string, args ...interface{}) { logImp(3, 2, "[ERROR]", fmt, args) }

func logImp(level int, layer int, prefix string, log_fmt string, args []interface{}) {
	if level < gConf.Base.iLogLevel {
		return
	}
	tmNow := time.Now()

	_, file, line, _ := runtime.Caller(layer)
	file = file[strings.LastIndexAny(file, "/\\")+1:]

	pre_fmt := "[%04d-%02d-%02d %02d:%02d:%02d.%03d][%s:%d]"
	pre_args := []interface{}{tmNow.Year(), tmNow.Month(), tmNow.Day(), tmNow.Hour(), tmNow.Minute(), tmNow.Second(), tmNow.Nanosecond() / 1000000, file, line}

	suffix := fmt.Sprintf("%04d%02d%02d", tmNow.Year(), tmNow.Month(), tmNow.Day())
	log := prefix + fmt.Sprintf(pre_fmt, pre_args...) + " " + fmt.Sprintf(log_fmt, args...) + "\n"

	gLog.WriteLog(suffix, log)
}

type Logger struct{}

func (self Logger) DEBUG_LOG(fmt string, args []interface{}) { logImp(1, 3, "[DEBUG]", fmt, args) }
func (self Logger) INFO_LOG(fmt string, args []interface{})  { logImp(2, 3, "[INFO]", fmt, args) }
func (self Logger) ERROR_LOG(fmt string, args []interface{}) { logImp(3, 3, "[ERROR]", fmt, args) }
