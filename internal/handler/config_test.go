package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/server"
	"msp/internal/storage"
)

func TestHandleConfig(t *testing.T) {
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

	s := server.New(tmpFile.Name(), nil)
	store := storage.NewStore(nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store, Favorites: store})

	t.Run("GET Config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		w := httptest.NewRecorder()

		h.HandleConfig(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp domain.ConfigResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
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

		var resp domain.ConfigResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if s.Config().Port != 9090 {
			t.Errorf("Expected port to be updated to 9090, got %d", s.Config().Port)
		}
	})
}