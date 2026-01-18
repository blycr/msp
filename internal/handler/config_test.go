package handler

import (
	"bytes"
	"encoding/json"
	"msp/internal/config"
	"msp/internal/server"
	"msp/internal/types"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandleConfig(t *testing.T) {
	// Setup temporary config file
	tmpFile, err := os.CreateTemp("", "config_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Errorf("remove temp config file: %v", err)
		}
	})
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	s := server.New(tmpFile.Name())
	h := New(s)

	t.Run("GET Config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		w := httptest.NewRecorder()

		h.HandleConfig(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp types.ConfigResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		// Since we used default config, check if default values are present
		// Note: ConfigResponse.Config is interface{}, so we need to be careful with assertions
		// In a real integration test we would check specific fields
	})

	t.Run("POST Config", func(t *testing.T) {
		newCfg := config.Default()
		newCfg.Port = 9090
		body, _ := json.Marshal(newCfg)
		req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
		w := httptest.NewRecorder()

		h.HandleConfig(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp types.ConfigResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		// Verify that the server config was updated
		if s.Config().Port != 9090 {
			t.Errorf("Expected port to be updated to 9090, got %d", s.Config().Port)
		}
	})
}
