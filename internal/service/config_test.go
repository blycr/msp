package service

import (
	"os"
	"path/filepath"
	"testing"

	"msp/internal/config"
	"msp/internal/server"
)

func TestConfigService_GetConfigView(t *testing.T) {
	// Setup temporary directory for config
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config_test.json")

	// Create Server
	srv := server.New(cfgPath)
	if err := srv.LoadOrInitConfig(); err != nil {
		t.Fatalf("Failed to init config: %v", err)
	}

	// Create Service
	svc := NewConfigService(srv)

	// Test GetConfigView
	view := svc.GetConfigView()

	if view.Config.Port != 8099 {
		t.Errorf("Expected default port 8099, got %d", view.Config.Port)
	}

	if len(view.URLs) == 0 {
		t.Error("Expected at least one URL (localhost)")
	}

	// Verify localhost URL is present
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
	// Setup
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config_test.json")
	srv := server.New(cfgPath)
	_ = srv.LoadOrInitConfig()
	svc := NewConfigService(srv)

	// Create a dummy directory for share test
	shareDir := filepath.Join(tmpDir, "My Videos")
	if err := os.MkdirAll(shareDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Prepare new config
	newCfg := srv.Config()
	newCfg.Port = 9000
	newCfg.Shares = []config.Share{
		{Label: "Test Share", Path: shareDir},
		{Label: "Bad Share", Path: "/path/to/nowhere"}, // Should be filtered out
	}

	// Test Update
	updated, err := svc.UpdateConfig(newCfg)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify return value
	if updated.Port != 9000 {
		t.Errorf("Expected port 9000, got %d", updated.Port)
	}
	if len(updated.Shares) != 1 {
		t.Errorf("Expected 1 valid share, got %d", len(updated.Shares))
	}
	if updated.Shares[0].Label != "Test Share" {
		t.Errorf("Expected share label 'Test Share', got %s", updated.Shares[0].Label)
	}

	// Verify persistence in Server
	current := srv.Config()
	if current.Port != 9000 {
		t.Errorf("Server config not updated, port is %d", current.Port)
	}
}
