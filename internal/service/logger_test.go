package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLoggerService(t *testing.T) {
	l := NewLoggerService("info", "/tmp/test.log")
	if l == nil {
		t.Fatal("NewLoggerService returned nil")
	}
	if l.cfgLevel != "info" {
		t.Errorf("expected cfgLevel info, got %s", l.cfgLevel)
	}
}

func TestLoggerServiceLog(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService("info", logPath)
	l.SetupLogger()
	defer l.Close()

	t.Run("info level logs info messages", func(t *testing.T) {
		l.Log(LogLevelInfo, "test info message")
	})

	t.Run("info level skips debug messages", func(t *testing.T) {
		l.Log(LogLevelDebug, "test debug message")
	})

	t.Run("error level always logs when not none", func(t *testing.T) {
		l.Log(LogLevelError, "test error message")
	})
}

func TestLoggerServiceLogLevelNone(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService(LogLevelNone, logPath)
	l.SetupLogger()
	defer l.Close()

	l.Log(LogLevelError, "should not appear")
	l.Log(LogLevelInfo, "should not appear")
}

func TestLoggerServiceLogDebug(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService(LogLevelDebug, logPath)
	l.SetupLogger()
	defer l.Close()

	l.Log(LogLevelDebug, "debug should appear")
	l.Log(LogLevelInfo, "info should appear")
	l.Log(LogLevelError, "error should appear")
}

func TestLoggerServiceSetupLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService("info", logPath)
	l.SetupLogger()
	defer l.Close()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file should exist after SetupLogger")
	}
}

func TestLoggerServiceRotateLogIfNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService("info", logPath)
	l.SetupLogger()
	defer l.Close()

	l.RotateLogIfNeeded()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file should still exist after rotation")
	}
}

func TestLoggerServiceLogRequest(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService("info", logPath)
	l.SetupLogger()
	defer l.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.100:1234"
	start := time.Now().Add(-10 * time.Millisecond)

	l.LogRequest(req, http.StatusOK, start)
}

func TestLoggerServiceLogRequest5xx(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService("info", logPath)
	l.SetupLogger()
	defer l.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/error", nil)
	req.RemoteAddr = "10.0.0.1:5678"
	start := time.Now()

	l.LogRequest(req, http.StatusInternalServerError, start)
}

func TestLoggerServiceUpdateConfig(t *testing.T) {
	l := NewLoggerService("info", "/tmp/old.log")

	l.UpdateConfig("debug", "/tmp/new.log")

	if l.cfgLevel != "debug" {
		t.Errorf("expected cfgLevel debug after update, got %s", l.cfgLevel)
	}
	if l.logFilePath != "/tmp/new.log" {
		t.Errorf("expected logFilePath /tmp/new.log after update, got %s", l.logFilePath)
	}
}

func TestLoggerServiceClose(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService("info", logPath)
	l.SetupLogger()
	l.Close()
}

func TestLoggerServiceLogRequestDefaultStatus(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService("info", logPath)
	l.SetupLogger()
	defer l.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:1111"
	start := time.Now()
	l.LogRequest(req, 0, start)
}

func TestLoggerServiceLogContainsLevelPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l := NewLoggerService(LogLevelDebug, logPath)
	l.SetupLogger()
	defer l.Close()

	l.Log(LogLevelInfo, "prefix-test")
	l.RotateLogIfNeeded()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	if !strings.Contains(string(data), "[INFO]") {
		t.Error("log should contain [INFO] prefix")
	}
}
