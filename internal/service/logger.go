package service

import (
	"context"
	"fmt"
	"log"
	"log/slog"
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

	// levelOff disables all logging (maps from LogLevelNone).
	levelOff = slog.Level(100)
)

type logEntry struct {
	level slog.Level
	msg   string
}

// toSlogLevel maps a facade level string to a slog level.
func toSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarning:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}

// cfgLevelToSlog maps the configured level to the minimum slog level.
// Unknown levels behave like the facade: error-only.
func cfgLevelToSlog(cfgLevel string) slog.Level {
	switch strings.ToLower(cfgLevel) {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarning:
		return slog.LevelWarn
	case LogLevelNone:
		return levelOff
	default:
		return slog.LevelError
	}
}

// fileWriter indirection: writes always go to the logger's current logFile,
// so log rotation can swap the file without rebuilding the slog handler.
type fileWriter struct {
	l *LoggerService
}

func (w fileWriter) Write(p []byte) (int, error) {
	w.l.logMu.Lock()
	defer w.l.logMu.Unlock()
	if w.l.logFile == nil {
		return len(p), nil
	}
	return w.l.logFile.Write(p)
}

// fanoutHandler dispatches records to the file handler and, for errors only,
// to the console handler (filtering context-cancellation noise), preserving
// the previous file+console behavior.
type fanoutHandler struct {
	file    slog.Handler
	console slog.Handler
}

func (h *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.file.Enabled(ctx, level) || h.console.Enabled(ctx, level)
}

func (h *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.file.Handle(ctx, r)
	if r.Level >= slog.LevelError &&
		!strings.Contains(r.Message, context.Canceled.Error()) &&
		!strings.Contains(r.Message, context.DeadlineExceeded.Error()) {
		if cerr := h.console.Handle(ctx, r); err == nil {
			err = cerr
		}
	}
	return err
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &fanoutHandler{file: h.file.WithAttrs(attrs), console: h.console.WithAttrs(attrs)}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	return &fanoutHandler{file: h.file.WithGroup(name), console: h.console.WithGroup(name)}
}

type LoggerService struct {
	mu          sync.RWMutex
	cfgLevel    string
	logFilePath string

	seenIPs sync.Map
	logMu   sync.Mutex
	logFile *os.File

	logger   *slog.Logger
	levelVar *slog.LevelVar

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
	cfgLevel := l.cfgLevel
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
	l.logMu.Unlock()

	levelVar := new(slog.LevelVar)
	levelVar.Set(cfgLevelToSlog(cfgLevel))

	fileHandler := slog.NewTextHandler(fileWriter{l: l}, &slog.HandlerOptions{Level: levelVar})
	consoleHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	l.logger = slog.New(&fanoutHandler{file: fileHandler, console: consoleHandler})
	l.levelVar = levelVar

	// Package-level slog calls share the same handler.
	slog.SetDefault(l.logger)

	// Redirect stdlib log output to the log file as a fallback for
	// third-party libraries. slog writes directly to the file (not via
	// stdlib log), so there is no duplicate output.
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

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

// writeLog writes a single log entry via slog. Called from writeLoop to keep
// all file I/O on one goroutine and avoid contention.
func (l *LoggerService) writeLog(entry logEntry) {
	if l.logger == nil {
		return
	}
	l.logger.Log(context.Background(), entry.level, entry.msg)
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

	select {
	case l.logChan <- logEntry{level: toSlogLevel(level), msg: msg}:
	default:
		// Channel full; drop the entry to avoid blocking callers.
	}
}

func (l *LoggerService) RotateLogIfNeeded() {
	l.logMu.Lock()

	if l.logFile == nil {
		l.logMu.Unlock()
		return
	}

	st, err := l.logFile.Stat()
	if err != nil {
		l.logMu.Unlock()
		return
	}

	if st.Size() < constants.LogRotateSize {
		l.logMu.Unlock()
		return
	}

	_ = l.logFile.Close()
	l.logFile = nil
	l.logMu.Unlock()

	l.mu.RLock()
	path := l.logFilePath
	l.mu.RUnlock()

	oldPath := path + ".1"
	_ = os.Remove(oldPath)
	_ = os.Rename(path, oldPath)

	f, err := openLogFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to reopen log file %s: %v; falling back to stderr\n", path, err)
		return
	}
	l.logMu.Lock()
	l.logFile = f
	l.logMu.Unlock()
	log.SetOutput(f)
}

// openLogFile opens (creating if needed) the log file at path for appending.
func openLogFile(path string) (*os.File, error) {
	//nolint:gosec // Log file path is controlled by config/CLI
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.FilePerm)
}

// Reopen closes the current log file and reopens it at the configured path.
// On failure the previous file handle is kept so logging continues, and an
// error is returned. It is a no-op when no log file path is configured.
func (l *LoggerService) Reopen() error {
	l.mu.RLock()
	path := l.logFilePath
	l.mu.RUnlock()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("reopen log: mkdir: %w", err)
	}
	f, err := openLogFile(path)
	if err != nil {
		return fmt.Errorf("reopen log %s: %w", path, err)
	}
	l.logMu.Lock()
	old := l.logFile
	l.logFile = f
	l.logMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	log.SetOutput(f)
	return nil
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
	if l.levelVar != nil {
		l.levelVar.Set(cfgLevelToSlog(cfgLevel))
	}
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
	}
}
