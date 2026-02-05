package handler

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"msp/internal/config"
	"msp/internal/db"
	"msp/internal/server"
)

func TestPINAuthentication(t *testing.T) {
	// Create a temporary directory for the test config
	tmpDir, err := os.MkdirTemp("", "msp-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		db.Close() // Close DB before removing temp dir
		os.RemoveAll(tmpDir)
	}()

	configPath := tmpDir + "/test_config.json"

	// Create a test server with PIN enabled
	s := server.New(configPath)
	err = s.UpdateConfig(func(cfg *config.Config) {
		cfg.Security.PINEnabled = true
		cfg.Security.PIN = "1234"
	})
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	h := New(s)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", h.HandleConfig)
	mux.HandleFunc("/api/media", h.HandleMedia)
	mux.HandleFunc("/api/pin", h.HandlePIN)

	// Wrap with security middleware
	secureHandler := WithSecurity(s, mux)

	// Create a test server
	server := httptest.NewServer(secureHandler)
	defer server.Close()

	// Create HTTP client with cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("Failed to create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
	}

	t.Run("Access config without auth", func(t *testing.T) {
		resp, err := client.Get(server.URL + "/api/config")
		if err != nil {
			t.Fatalf("Failed to get config: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Access media without auth", func(t *testing.T) {
		resp, err := client.Get(server.URL + "/api/media")
		if err != nil {
			t.Fatalf("Failed to get media: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Login with wrong PIN", func(t *testing.T) {
		resp, err := client.Post(server.URL+"/api/pin", "application/json", strings.NewReader(`{"pin":"wrong"}`))
		if err != nil {
			t.Fatalf("Failed to post PIN: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Login with correct PIN", func(t *testing.T) {
		resp, err := client.Post(server.URL+"/api/pin", "application/json", strings.NewReader(`{"pin":"1234"}`))
		if err != nil {
			t.Fatalf("Failed to post PIN: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Check if cookie was set
		cookies := jar.Cookies(resp.Request.URL)
		found := false
		for _, c := range cookies {
			if c.Name == "msp_session" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Session cookie not set")
		}
	})

	t.Run("Access media with auth", func(t *testing.T) {
		resp, err := client.Get(server.URL + "/api/media")
		if err != nil {
			t.Fatalf("Failed to get media: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
