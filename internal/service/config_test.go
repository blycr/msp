package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"msp/internal/config"
	"msp/internal/domain"
)

type mockConfigProvider struct {
	mu  sync.RWMutex
	cfg config.Config
}

func (m *mockConfigProvider) Config() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *mockConfigProvider) UpdateConfig(fn func(*config.Config)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(&m.cfg)
	return nil
}

func (m *mockConfigProvider) GetPort() int {
	return m.cfg.Port
}

type mockCacheInvalidator struct{}

func (m *mockCacheInvalidator) InvalidateMediaCache() {}

func TestConfigService_GetConfigView(t *testing.T) {
	cfg := config.Default()
	cfg.Port = 8099

	mock := &mockConfigProvider{cfg: cfg}
	svc := NewConfigService(mock, &mockCacheInvalidator{})

	view := svc.GetConfigView()

	if view.Config.Port != 8099 {
		t.Errorf("Expected default port 8099, got %d", view.Config.Port)
	}

	if len(view.URLs) == 0 {
		t.Error("Expected at least one URL (localhost)")
	}

	foundLocal := false
	for _, u := range view.URLs {
		if u == "http://127.0.0.1:8099/" {
			foundLocal = true
			break
		}
	}
	if !foundLocal {
		t.Error("Expected localhost URL in view")
	}
}

func TestConfigService_UpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	shareDir := filepath.Join(tmpDir, "My Videos")
	if err := os.MkdirAll(shareDir, 0750); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	mock := &mockConfigProvider{cfg: cfg}
	svc := NewConfigService(mock, &mockCacheInvalidator{})

	newCfg := mock.Config()
	newCfg.Port = 9000
	newCfg.Shares = []domain.Share{
		{Label: "Test Share", Path: shareDir},
		{Label: "Bad Share", Path: "/path/to/nowhere"},
	}

	updated, err := svc.UpdateConfig(newCfg)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	if updated.Port != 9000 {
		t.Errorf("Expected port 9000, got %d", updated.Port)
	}
	if len(updated.Shares) != 1 {
		t.Errorf("Expected 1 valid share, got %d", len(updated.Shares))
	}
	if updated.Shares[0].Label != "Test Share" {
		t.Errorf("Expected share label 'Test Share', got %s", updated.Shares[0].Label)
	}

	current := mock.Config()
	if current.Port != 9000 {
		t.Errorf("Config not updated, port is %d", current.Port)
	}
}