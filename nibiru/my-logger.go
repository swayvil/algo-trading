package nibiru

import (
	log "log"
	"os"
	"sync"
)

type MyLogger struct {
	console     *log.Logger
	//ordersBooks *log.Logger
}

var instanceLogger *MyLogger
var onceLogger sync.Once

func GetLoggerInstance() *MyLogger {
	onceLogger.Do(func() {
		instanceLogger = &MyLogger{}
		instanceLogger.initLog("")
	})
	return instanceLogger
}

func (myLogger *MyLogger) initLog(prefix string) {
	myLogger.deleteFile(GetConfigInstance().ConsoleLog)
	myLogger.console = log.New(myLogger.openFile(GetConfigInstance().ConsoleLog), prefix, log.Lshortfile|log.LstdFlags)
}

func (myLogger *MyLogger) openFile(path string) *os.File {
	f2, err2 := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err2 != nil {
		GetLoggerInstance().Error("In opening file: %s. %s", path, err2.Error())
		os.Exit(1)
	}
	return f2
}

func (myLogger *MyLogger) deleteFile(path string) {
	if _, err := os.Stat(path); os.IsExist(err) {
		var err = os.Remove(path)
		if err != nil {
			GetLoggerInstance().Error("In deletring file: %s. %s", path, err.Error())
			os.Exit(1)
		}
	}
}

func (myLogger *MyLogger) Info(msg string, args ...interface{}) {
	myLogger.console.Printf("[INFO] " + msg + "\n", args...)
}

func (myLogger *MyLogger) Error(msg string, args ...interface{}) {
	myLogger.console.Printf("[ERROR] " + msg + "\n", args...)
}
