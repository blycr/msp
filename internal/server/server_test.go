package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"msp/internal/config"
)

func TestServerNew(t *testing.T) {
	s := New("config.json")
	if s == nil {
		t.Fatal("Expected server New to not be nil")
	}
}

func TestLoadOrInitConfig_CreatesDefaultWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	s := New(cfgPath)
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

	s := New(cfgPath)
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

	s := New(cfgPath)
	err := s.LoadOrInitConfig()
	if err == nil {
		t.Error("LoadOrInitConfig should return error for invalid JSON")
	}
}

func TestConfig(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(filepath.Join(tmpDir, "config.json"))
	cfg := s.Config()
	// Config() 应返回当前（零值）配置而不 panic
	_ = cfg
}

func TestUpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	s := New(cfgPath)
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
	s := New("config.json")
	// 未设置 Port 时应返回 DefaultPort
	port := s.GetPort()
	if port != config.DefaultPort {
		t.Errorf("expected DefaultPort %d, got %d", config.DefaultPort, port)
	}
}

func TestGetPort_ReturnsConfiguredPort(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	s := New(cfgPath)
	_ = s.UpdateConfig(func(cfg *config.Config) {
		cfg.Port = 5555
	})
	if s.GetPort() != 5555 {
		t.Errorf("expected port 5555, got %d", s.GetPort())
	}
}

func TestCreateAndValidateSession(t *testing.T) {
	s := New("config.json")

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
	s := New("config.json")
	if s.ValidateSession("") {
		t.Error("empty token should be invalid")
	}
}

func TestValidateSession_Unknown(t *testing.T) {
	s := New("config.json")
	if s.ValidateSession("unknowntokenxyz") {
		t.Error("unknown token should be invalid")
	}
}

func TestLog_DoesNotPanic(t *testing.T) {
	s := New("config.json")
	// 确保调用 Log 不 panic（未初始化 logger 文件的情况）
	s.Log("info", "test message")
	s.Log("error", "test error")
}
