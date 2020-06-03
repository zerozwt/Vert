package main

import (
	"os"
)

type logUnit struct {
	suffix string
	log    string
}

var logCh chan logUnit = make(chan logUnit, 16384)
var logStop chan bool = make(chan bool)
var logJoin chan bool = make(chan bool)

var logSuffix string = ""
var logFile *os.File = nil

func doLog(unit logUnit) {
	if unit.suffix != logSuffix {
		if logFile != nil {
			logFile.Close()
		}
		logSuffix = unit.suffix
		logFile = nil

		if tmp, err := os.OpenFile(gConf.Base.LogFile+"."+logSuffix, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644); err != nil {
			return
		} else {
			logFile = tmp
		}
	}
	logFile.Write([]byte(unit.log))
}

func logLoop() {
	for {
		select {
		case <-logStop:
			if logFile != nil {
				logFile.Close()
			}
			close(logJoin)
			return
		case unit := <-logCh:
			doLog(unit)
		}
	}
}

func joinLog() {
	close(logStop)
	<-logJoin
}

func initLog() {
	go logLoop()
}

type logWriter struct{}

func (self logWriter) WriteLog(suffix, log string) {
	logCh <- logUnit{suffix, log}
}

var gLog logWriter
