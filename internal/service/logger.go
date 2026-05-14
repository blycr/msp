package service

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"msp/internal/constants"
	"msp/internal/util"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelError = "error"
	LogLevelNone  = "none"
)

type LoggerService struct {
	mu      sync.RWMutex
	cfgLevel string
	logFilePath string

	seenIPs sync.Map
	logMu   sync.Mutex
	logFile *os.File
	logCnt  int32
}

func NewLoggerService(cfgLevel, logFilePath string) *LoggerService {
	return &LoggerService{
		cfgLevel:    strings.ToLower(cfgLevel),
		logFilePath: logFilePath,
	}
}

func (l *LoggerService) SetupLogger() {
	l.mu.Lock()
	if l.logFilePath == "" {
		l.logFilePath = filepath.Join(util.MustExeDir(), "logs", "msp.log")
	}
	logFile := l.logFilePath
	l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(logFile), 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		return
	}

	l.logMu.Lock()
	defer l.logMu.Unlock()

	if l.logFile != nil {
		_ = l.logFile.Close()
	}

	//nolint:gosec // Log file path is controlled by config/CLI
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.FilePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	l.logFile = f
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func (l *LoggerService) Log(level string, msg string) {
	l.mu.RLock()
	cfgLevel := l.cfgLevel
	l.mu.RUnlock()

	shouldLog := false
	switch strings.ToLower(level) {
	case LogLevelError:
		shouldLog = cfgLevel != LogLevelNone
	case LogLevelInfo:
		shouldLog = cfgLevel == LogLevelInfo || cfgLevel == LogLevelDebug
	case LogLevelDebug:
		shouldLog = cfgLevel == LogLevelDebug
	}

	if shouldLog {
		ts := time.Now().Format(constants.LogTimeFormat)
		line := fmt.Sprintf("%s [%s] %s", ts, strings.ToUpper(level), msg)
		log.Println(line)

		if cnt := atomic.AddInt32(&l.logCnt, 1); cnt%constants.LogRotateCheckInterval == 0 {
			l.RotateLogIfNeeded()
		}
	}
}

func (l *LoggerService) RotateLogIfNeeded() {
	l.logMu.Lock()
	defer l.logMu.Unlock()

	if l.logFile == nil {
		return
	}

	st, err := l.logFile.Stat()
	if err != nil {
		return
	}

	if st.Size() < constants.LogRotateSize {
		return
	}

	_ = l.logFile.Close()
	l.logFile = nil

	l.mu.RLock()
	path := l.logFilePath
	l.mu.RUnlock()

	oldPath := path + ".1"
	_ = os.Remove(oldPath)
	_ = os.Rename(path, oldPath)

	//nolint:gosec // Log file path is controlled by config/CLI
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.FilePerm)
	if err != nil {
		log.Printf("[ERROR] Failed to reopen log file %s: %v; falling back to stderr", path, err)
		log.SetOutput(os.Stderr)
		return
	}
	l.logFile = f
	log.SetOutput(f)
}

func (l *LoggerService) LogRequest(r *http.Request, status int, start time.Time) {
	if status == 0 {
		status = http.StatusOK
	}
	ua := strings.TrimSpace(r.UserAgent())
	duration := time.Since(start).Milliseconds()

	msg := fmt.Sprintf("%s %s status=%d ua=%s ms=%d", r.Method, r.URL.Path, status, ua, duration)

	level := LogLevelInfo
	if status >= 500 {
		level = LogLevelError
	}
	l.Log(level, msg)

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	if ip != "" && ip != constants.LocalhostIPv4 && ip != constants.LocalhostIPv6 {
		if _, seen := l.seenIPs.Load(ip); !seen {
			l.seenIPs.Store(ip, true)
			l.Log(LogLevelInfo, fmt.Sprintf("[NEW DEVICE] %s %s", ip, msg))
		}
	}
}

func (l *LoggerService) UpdateConfig(cfgLevel, logFilePath string) {
	l.mu.Lock()
	l.cfgLevel = strings.ToLower(cfgLevel)
	l.logFilePath = logFilePath
	l.mu.Unlock()
}

func (l *LoggerService) Close() {
	l.logMu.Lock()
	defer l.logMu.Unlock()
	if l.logFile != nil {
		_ = l.logFile.Close()
		l.logFile = nil
	}
}
