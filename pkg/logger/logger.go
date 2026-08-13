package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level string

const (
	TagInfo      Level = "INFO"
	TagWarn      Level = "WARN"
	TagError     Level = "ERROR"
	TagSuccess   Level = "SUCCESS"
	TagMCServer  Level = "MC-SERVER"
	TagHTTP      Level = "HTTP"
	TagWebSocket Level = "WEBSOCKET"
	TagFileMgr   Level = "FILEMGR"
	TagAuth      Level = "AUTH"
)

type Logger struct {
	mu          sync.Mutex
	logDir      string
	logFile     *os.File
	currentDate string
	writer      io.Writer
}

var globalLogger *Logger

func init() {
	globalLogger = &Logger{
		logDir: "logs",
	}
	_ = globalLogger.initLogFile()
}

func GetLogger() *Logger {
	return globalLogger
}

func (l *Logger) initLogFile() error {
	now := time.Now()
	dateFolder := now.Format("2006-01-02")
	timeFileName := now.Format("15-04-05") + ".log"

	dirPath := filepath.Join(l.logDir, dateFolder)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dirPath, timeFileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.logFile = file
	l.currentDate = dateFolder
	l.writer = io.MultiWriter(os.Stdout, file)

	return nil
}

func (l *Logger) checkRotate() {
	now := time.Now()
	dateFolder := now.Format("2006-01-02")
	if dateFolder != l.currentDate {
		if l.logFile != nil {
			_ = l.logFile.Close()
		}
		_ = l.initLogFile()
	}
}

func (l *Logger) Log(tag Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.checkRotate()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	entry := fmt.Sprintf("[%s] [%-9s] %s\n", timestamp, tag, message)

	if l.writer != nil {
		_, _ = l.writer.Write([]byte(entry))
	} else {
		fmt.Print(entry)
	}
}

func Info(format string, args ...interface{}) {
	globalLogger.Log(TagInfo, format, args...)
}

func Warn(format string, args ...interface{}) {
	globalLogger.Log(TagWarn, format, args...)
}

func Error(format string, args ...interface{}) {
	globalLogger.Log(TagError, format, args...)
}

func Success(format string, args ...interface{}) {
	globalLogger.Log(TagSuccess, format, args...)
}

func MCServer(format string, args ...interface{}) {
	globalLogger.Log(TagMCServer, format, args...)
}

func HTTPLog(format string, args ...interface{}) {
	globalLogger.Log(TagHTTP, format, args...)
}

func WSLog(format string, args ...interface{}) {
	globalLogger.Log(TagWebSocket, format, args...)
}

func FileLog(format string, args ...interface{}) {
	globalLogger.Log(TagFileMgr, format, args...)
}

func AuthLog(format string, args ...interface{}) {
	globalLogger.Log(TagAuth, format, args...)
}
