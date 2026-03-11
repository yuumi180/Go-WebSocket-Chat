package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Logger struct {
	infoLog  *log.Logger
	errorLog *log.Logger
	warnLog  *log.Logger
}

var Log *Logger

func Init() {
	// 创建 logs 目录
	os.MkdirAll("logs", os.ModePerm)

	filename := time.Now().Format("2006-01-02") + ".log"
	filepath := filepath.Join("logs", filename)

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}

	Log = &Logger{
		infoLog:  log.New(file, "[INFO] ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLog: log.New(file, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile),
		warnLog:  log.New(file, "[WARN] ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func Info(v ...interface{}) {
	Log.infoLog.Print(v...)
}

func Error(v ...interface{}) {
	Log.errorLog.Print(v...)
}

func Warn(v ...interface{}) {
	Log.warnLog.Print(v...)
}

func GetCallerInfo() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return ""
	}
	return filepath.Base(file) + ":" + fmt.Sprintf("%d", line)
}
