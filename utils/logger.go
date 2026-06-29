package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "INFO"
	}
}

func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// Logger 日志器
type Logger struct {
	infoLog    *log.Logger
	errorLog   *log.Logger
	accessLog  *log.Logger
	debugLog   *log.Logger
	consoleLog *log.Logger
	level      LogLevel
	logDir     string
	infoFile   *os.File
	errorFile  *os.File
	accessFile *os.File
	debugFile  *os.File
}

// NewLogger 创建日志器
func NewLogger(logDir string, level LogLevel) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	dateStr := time.Now().Format("2006-01-02")

	debugFile, err := os.OpenFile(
		filepath.Join(logDir, fmt.Sprintf("debug-%s.log", dateStr)),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		return nil, fmt.Errorf("创建debug日志文件失败: %w", err)
	}

	infoFile, err := os.OpenFile(
		filepath.Join(logDir, fmt.Sprintf("info-%s.log", dateStr)),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		return nil, fmt.Errorf("创建info日志文件失败: %w", err)
	}

	errorFile, err := os.OpenFile(
		filepath.Join(logDir, fmt.Sprintf("error-%s.log", dateStr)),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		return nil, fmt.Errorf("创建error日志文件失败: %w", err)
	}

	accessFile, err := os.OpenFile(
		filepath.Join(logDir, fmt.Sprintf("access-%s.log", dateStr)),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		return nil, fmt.Errorf("创建access日志文件失败: %w", err)
	}

	consoleWriter := os.Stderr

	return &Logger{
		infoLog:    log.New(io.MultiWriter(infoFile, consoleWriter), "", 0),
		errorLog:   log.New(io.MultiWriter(errorFile, consoleWriter), "", 0),
		accessLog:  log.New(io.MultiWriter(accessFile, consoleWriter), "", 0),
		debugLog:   log.New(io.MultiWriter(debugFile, consoleWriter), "", 0),
		consoleLog: log.New(consoleWriter, "", 0),
		level:      level,
		logDir:     logDir,
		infoFile:   infoFile,
		errorFile:  errorFile,
		accessFile: accessFile,
		debugFile:  debugFile,
	}, nil
}

// Close 关闭所有日志文件
func (l *Logger) Close() {
	if l.infoFile != nil {
		l.infoFile.Close()
	}
	if l.errorFile != nil {
		l.errorFile.Close()
	}
	if l.accessFile != nil {
		l.accessFile.Close()
	}
	if l.debugFile != nil {
		l.debugFile.Close()
	}
}

// formatTimestamp 获取格式化的 timestamp
func formatTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}

// Debugf 输出调试日志
func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.level > DEBUG {
		return
	}
	msg := fmt.Sprintf(format, v...)
	l.debugLog.Printf("[DEBUG] [%s] %s", formatTimestamp(), msg)
}

// Infof 输出信息日志
func (l *Logger) Infof(format string, v ...interface{}) {
	if l.level > INFO {
		return
	}
	msg := fmt.Sprintf(format, v...)
	l.infoLog.Printf("[INFO] [%s] %s", formatTimestamp(), msg)
}

// Warnf 输出警告日志
func (l *Logger) Warnf(format string, v ...interface{}) {
	if l.level > WARN {
		return
	}
	msg := fmt.Sprintf(format, v...)
	l.infoLog.Printf("[WARN] [%s] %s", formatTimestamp(), msg)
}

// Errorf 输出错误日志
func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.level > ERROR {
		return
	}
	msg := fmt.Sprintf(format, v...)
	l.errorLog.Printf("[ERROR] [%s] %s", formatTimestamp(), msg)
}

// Accessf 输出访问日志 (包含来源IP和用户名)
func (l *Logger) Accessf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.accessLog.Printf("[ACCESS] [%s] %s", formatTimestamp(), msg)
}

// GetLevel 获取日志级别
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// GetLogDir 获取日志目录
func (l *Logger) GetLogDir() string {
	return l.logDir
}

// AccessContext 访问上下文
type AccessContext struct {
	Logger    *Logger
	ClientIP  string
	Username  string
	AuthOK    bool
	Method    string
	Path      string
	StartTime time.Time
}

// NewAccessContext 创建访问上下文
func NewAccessContext(logger *Logger, clientIP, username string, authOK bool) *AccessContext {
	return &AccessContext{
		Logger:    logger,
		ClientIP:  clientIP,
		Username:  username,
		AuthOK:    authOK,
		StartTime: time.Now(),
	}
}

// LogAuthSuccess 记录认证成功
func (ac *AccessContext) LogAuthSuccess() {
	ac.Logger.Accessf("AUTH SUCCESS | IP: %-15s | User: %s", ac.ClientIP, ac.Username)
}

// LogAuthFailed 记录认证失败
func (ac *AccessContext) LogAuthFailed(reason string) {
	ac.Logger.Accessf("AUTH FAILED | IP: %-15s | User: %s | Reason: %s", ac.ClientIP, ac.Username, reason)
}

// LogQuery 记录查询请求
func (ac *AccessContext) LogQuery(toolName string, query string, args map[string]interface{}) {
	argsJSON, _ := json.Marshal(args)
	ac.Logger.Accessf("QUERY | IP: %-15s | User: %-15s | Tool: %-12s | Query: %s | Args: %s",
		ac.ClientIP, ac.Username, toolName, query, string(argsJSON))
}

// LogQueryResult 记录查询结果
func (ac *AccessContext) LogQueryResult(toolName string, success bool, duration time.Duration, resultPreview string) {
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	ac.Logger.Accessf("RESULT | IP: %-15s | User: %-15s | Tool: %-12s | Status: %-7s | Duration: %-10v | Result: %s",
		ac.ClientIP, ac.Username, toolName, status, duration, resultPreview)
}

// LogRequest 记录一般请求
func (ac *AccessContext) LogRequest(method, path string) {
	ac.Logger.Accessf("REQUEST | IP: %-15s | User: %-15s | Method: %-8s | Path: %s",
		ac.ClientIP, ac.Username, method, path)
}
