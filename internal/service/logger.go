package service

import (
	"context"
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
	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
	LogLevelNone    = "none"

	logChanCapacity = 4096
)

type logEntry struct {
	level string
	line  string
}

type LoggerService struct {
	mu      sync.RWMutex
	cfgLevel string
	logFilePath string

	seenIPs sync.Map
	logMu   sync.Mutex
	logFile *os.File

	fileLogger    *log.Logger
	consoleLogger *log.Logger

	logChan chan logEntry
	done    chan struct{}
	wg      sync.WaitGroup
}

func NewLoggerService(cfgLevel, logFilePath string) *LoggerService {
	return &LoggerService{
		cfgLevel:    strings.ToLower(cfgLevel),
		logFilePath: logFilePath,
		logChan:     make(chan logEntry, logChanCapacity),
		done:        make(chan struct{}),
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

	if l.logFile != nil {
		_ = l.logFile.Close()
	}

	//nolint:gosec // Log file path is controlled by config/CLI
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.FilePerm)
	if err != nil {
		l.logMu.Unlock()
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	l.logFile = f
	l.fileLogger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	l.consoleLogger = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	l.logMu.Unlock()

	// Start the background log writer goroutine.
	l.wg.Add(1)
	go l.writeLoop()
}

func (l *LoggerService) writeLoop() {
	defer l.wg.Done()
	var writeCnt int32
	for {
		select {
		case <-l.done:
			// Drain remaining entries.
			for {
				select {
				case entry := <-l.logChan:
					l.writeLog(entry)
				default:
					return
				}
			}
		case entry := <-l.logChan:
			l.writeLog(entry)
			if cnt := atomic.AddInt32(&writeCnt, 1); cnt%constants.LogRotateCheckInterval == 0 {
				l.RotateLogIfNeeded()
			}
		}
	}
}

// writeLog writes a single log entry to file and optionally to console.
// Called from writeLoop to keep all file I/O on one goroutine and avoid contention.
func (l *LoggerService) writeLog(entry logEntry) {
	l.logMu.Lock()

	if l.fileLogger != nil {
		l.fileLogger.Println(entry.line)
	}

	// Only error level (and above) goes to console, filtering context cancellation noise.
	if entry.level == LogLevelError && l.consoleLogger != nil {
		if !strings.Contains(entry.line, context.Canceled.Error()) && !strings.Contains(entry.line, context.DeadlineExceeded.Error()) {
			l.consoleLogger.Println(entry.line)
		}
	}

	l.logMu.Unlock()
}

func (l *LoggerService) Log(level string, msg string) {
	l.mu.RLock()
	cfgLevel := l.cfgLevel
	l.mu.RUnlock()

	shouldLog := false
	switch strings.ToLower(level) {
	case LogLevelError:
		shouldLog = cfgLevel != LogLevelNone
	case LogLevelWarning:
		shouldLog = cfgLevel == LogLevelDebug || cfgLevel == LogLevelInfo || cfgLevel == LogLevelWarning
	case LogLevelInfo:
		shouldLog = cfgLevel == LogLevelInfo || cfgLevel == LogLevelDebug
	case LogLevelDebug:
		shouldLog = cfgLevel == LogLevelDebug
	}

	if !shouldLog {
		return
	}

	line := fmt.Sprintf("[%s] %s", strings.ToUpper(level), msg)

	select {
	case l.logChan <- logEntry{level: level, line: line}:
	default:
		// Channel full; drop the entry to avoid blocking callers.
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
	l.fileLogger = nil

	l.mu.RLock()
	path := l.logFilePath
	l.mu.RUnlock()

	oldPath := path + ".1"
	_ = os.Remove(oldPath)
	_ = os.Rename(path, oldPath)

	//nolint:gosec // Log file path is controlled by config/CLI
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.FilePerm)
	if err != nil {
		log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds).Printf("[ERROR] Failed to reopen log file %s: %v; falling back to stderr", path, err)
		return
	}
	l.logFile = f
	l.fileLogger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
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
	// Signal the write loop to stop and wait for it to drain remaining entries.
	close(l.done)
	l.wg.Wait()

	l.logMu.Lock()
	defer l.logMu.Unlock()
	if l.logFile != nil {
		_ = l.logFile.Close()
		l.logFile = nil
		l.fileLogger = nil
	}
}
