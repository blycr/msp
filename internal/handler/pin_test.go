package handler

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"msp/internal/config"
	"msp/internal/server"
	"msp/internal/storage"
)

func TestPINAuthentication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "msp-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	configPath := tmpDir + "/test_config.json"

	s := server.New(configPath, nil)
	err = s.UpdateConfig(func(cfg *config.Config) {
		cfg.Security.PINEnabled = true
		cfg.Security.PIN = "1234"
	})
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	store := storage.NewStore(nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", h.HandleConfig)
	mux.HandleFunc("/api/media", h.HandleMedia)
	mux.HandleFunc("/api/pin", h.HandlePIN)

	secureHandler := WithSecurity(s, s, s, mux)

	srv := httptest.NewServer(secureHandler)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("Failed to create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
	}

	t.Run("Access config without auth", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/api/config")
		if err != nil {
			t.Fatalf("Failed to get config: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Access media without auth", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/api/media")
		if err != nil {
			t.Fatalf("Failed to get media: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Login with wrong PIN", func(t *testing.T) {
		resp, err := client.Post(srv.URL+"/api/pin", "application/json", strings.NewReader(`{"pin":"wrong"}`))
		if err != nil {
			t.Fatalf("Failed to post PIN: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Login with correct PIN", func(t *testing.T) {
		resp, err := client.Post(srv.URL+"/api/pin", "application/json", strings.NewReader(`{"pin":"1234"}`))
		if err != nil {
			t.Fatalf("Failed to post PIN: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		cookies := jar.Cookies(resp.Request.URL)
		found := false
		for _, c := range cookies {
			if c.Name == "msp_session" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Session cookie not found")
		}
	})

	t.Run("Access media with auth", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/api/media")
		if err != nil {
			t.Fatalf("Failed to get media: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}