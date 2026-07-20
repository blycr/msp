package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"msp/internal/config"
	"msp/internal/domain"
)

func TestServerNew(t *testing.T) {
	s := New("config.json", nil)
	if s == nil {
		t.Fatal("Expected server New to not be nil")
	}
}

func TestLoadOrInitConfig_CreatesDefaultWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	s := New(cfgPath, nil)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig error: %v", err)
	}

	// 文件应已被创建
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("config file should be created when missing")
	}

	// 加载后的配置应有合理的默认值
	cfg := s.Config()
	if cfg.Port <= 0 {
		t.Errorf("expected positive port, got %d", cfg.Port)
	}
}

func TestLoadOrInitConfig_LoadsExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	// 先写入一个包含自定义 port 的配置
	existing := config.Default()
	existing.Port = 9988
	b, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(cfgPath, b, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	s := New(cfgPath, nil)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig error: %v", err)
	}

	if s.Config().Port != 9988 {
		t.Errorf("expected port 9988, got %d", s.Config().Port)
	}
}

func TestLoadOrInitConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte("not-json"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	s := New(cfgPath, nil)
	err := s.LoadOrInitConfig()
	if err == nil {
		t.Error("LoadOrInitConfig should return error for invalid JSON")
	}
}

func TestConfig(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(filepath.Join(tmpDir, "config.json"), nil)
	cfg := s.Config()
	// Config() 应返回当前（零值）配置而不 panic
	_ = cfg
}

func TestUpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	s := New(cfgPath, nil)
	err := s.UpdateConfig(func(cfg *config.Config) {
		cfg.Port = 7777
	})
	if err != nil {
		t.Fatalf("UpdateConfig error: %v", err)
	}

	if s.Config().Port != 7777 {
		t.Errorf("expected port 7777 after update, got %d", s.Config().Port)
	}

	// 验证文件也已持久化
	b, err := os.ReadFile(cfgPath) //nolint:gosec // G304: testing with temp file is safe
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var saved config.Config
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if saved.Port != 7777 {
		t.Errorf("persisted config port should be 7777, got %d", saved.Port)
	}
}

func TestGetPort_DefaultWhenZero(t *testing.T) {
	s := New("config.json", nil)
	// 未设置 Port 时应返回 DefaultPort
	port := s.GetPort()
	if port != config.DefaultPort {
		t.Errorf("expected DefaultPort %d, got %d", config.DefaultPort, port)
	}
}

func TestGetPort_ReturnsConfiguredPort(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	s := New(cfgPath, nil)
	_ = s.UpdateConfig(func(cfg *config.Config) {
		cfg.Port = 5555
	})
	if s.GetPort() != 5555 {
		t.Errorf("expected port 5555, got %d", s.GetPort())
	}
}

func TestCreateAndValidateSession(t *testing.T) {
	s := New("config.json", nil)

	token, err := s.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}
	if !s.ValidateSession(token) {
		t.Error("newly created session should be valid")
	}
}

func TestValidateSession_Empty(t *testing.T) {
	s := New("config.json", nil)
	if s.ValidateSession("") {
		t.Error("empty token should be invalid")
	}
}

func TestValidateSession_Unknown(t *testing.T) {
	s := New("config.json", nil)
	if s.ValidateSession("unknowntokenxyz") {
		t.Error("unknown token should be invalid")
	}
}

func TestLog_DoesNotPanic(t *testing.T) {
	s := New("config.json", nil)
	// 确保调用 Log 不 panic（未初始化 logger 文件的情况）
	s.Log("info", "test message")
	s.Log("error", "test error")
}

// writeConfigAndReload persists cfg to cfgPath and forces checkAndReloadConfig
// to pick it up regardless of filesystem mtime granularity.
func writeConfigAndReload(t *testing.T, s *Server, cfgPath string, cfg config.Config) {
	t.Helper()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}
	if err := os.WriteFile(cfgPath, b, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	s.mu.Lock()
	s.cfgModTime = time.Now().Add(-time.Hour)
	s.mu.Unlock()
	s.checkAndReloadConfig()
}

func TestCheckAndReload_LogFileChangeReopensLog(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	oldLog := filepath.Join(tmpDir, "old.log")
	newLog := filepath.Join(tmpDir, "new.log")

	cfg := config.Default()
	cfg.LogFile = oldLog
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, b, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	s := New(cfgPath, nil)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig error: %v", err)
	}
	s.SetupLogger()
	defer s.logger.Close()

	cfg.LogFile = newLog
	writeConfigAndReload(t, s, cfgPath, cfg)

	if got := s.Config().LogFile; got != newLog {
		t.Errorf("expected LogFile %q after reload, got %q", newLog, got)
	}
	// Reopen 应已创建新日志文件
	if _, err := os.Stat(newLog); err != nil {
		t.Fatalf("new log file should be created by Reopen: %v", err)
	}

	// 写入一条日志，验证其落到新文件
	const marker = "reload-marker-xyz"
	s.Log("error", marker)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(newLog) //nolint:gosec // G304: testing with temp file is safe
		if strings.Contains(string(data), marker) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("marker %q not found in new log file after Reopen", marker)
}

func TestCheckAndReload_SharesChangeTriggersRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Default()
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, b, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	s := New(cfgPath, nil)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig error: %v", err)
	}

	var refreshCount int32
	s.refreshMedia = func(config.Config) { atomic.AddInt32(&refreshCount, 1) }

	cfg.Shares = []domain.Share{{Path: tmpDir, Label: "media"}}
	writeConfigAndReload(t, s, cfgPath, cfg)

	if got := atomic.LoadInt32(&refreshCount); got != 1 {
		t.Errorf("expected media refresh to be triggered once, got %d", got)
	}
	if got := s.Config().Shares; len(got) != 1 || got[0].Label != "media" {
		t.Errorf("shares not replaced after reload: %+v", got)
	}

	// Shares 未变时不应再次触发
	writeConfigAndReload(t, s, cfgPath, cfg)
	if got := atomic.LoadInt32(&refreshCount); got != 1 {
		t.Errorf("expected no extra refresh when shares unchanged, got %d", got)
	}
}

func TestUpdateConfig_SharesChangeTriggersRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(filepath.Join(tmpDir, "config.json"), nil)

	var refreshCount int32
	s.refreshMedia = func(config.Config) { atomic.AddInt32(&refreshCount, 1) }

	if err := s.UpdateConfig(func(cfg *config.Config) {
		cfg.Shares = []domain.Share{{Path: tmpDir, Label: "x"}}
	}); err != nil {
		t.Fatalf("UpdateConfig error: %v", err)
	}
	if got := atomic.LoadInt32(&refreshCount); got != 1 {
		t.Errorf("expected media refresh to be triggered once via UpdateConfig, got %d", got)
	}
}

func TestCheckAndReload_PortChangeWarnOnly(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Default()
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, b, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	s := New(cfgPath, nil)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig error: %v", err)
	}

	var refreshCount int32
	s.refreshMedia = func(config.Config) { atomic.AddInt32(&refreshCount, 1) }

	cfg.Port = 9090
	writeConfigAndReload(t, s, cfgPath, cfg) // 应仅 WARN，不 panic

	if got := s.Config().Port; got != 9090 {
		t.Errorf("expected port 9090 after reload, got %d", got)
	}
	if got := atomic.LoadInt32(&refreshCount); got != 0 {
		t.Errorf("port change should not trigger media refresh, got %d", got)
	}
}

func TestCheckAndReload_EncodingChangeWarnOnly(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Default()
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, b, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	s := New(cfgPath, nil)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig error: %v", err)
	}

	cfg.Playback.Video.Encoding = &config.TranscodeConfig{HWAccel: "none", MaxJobs: 2}
	writeConfigAndReload(t, s, cfgPath, cfg) // 应仅 WARN，不 panic

	enc := s.Config().Playback.Video.Encoding
	if enc == nil || enc.HWAccel != "none" || enc.MaxJobs != 2 {
		t.Errorf("encoding config not replaced after reload: %+v", enc)
	}
}
