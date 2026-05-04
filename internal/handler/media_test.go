package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"msp/internal/domain"
	"msp/internal/server"
	"msp/internal/storage"
)

func setupTestHandler(t *testing.T) (*Handler, *server.Server) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	s := server.New(configPath)
	store := storage.NewStore(nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store})
	return h, s
}

func TestHandleMediaGet(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	w := httptest.NewRecorder()

	h.HandleMedia(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp domain.MediaResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Videos == nil {
		t.Error("videos should not be nil")
	}
	if resp.Audios == nil {
		t.Error("audios should not be nil")
	}
}

func TestHandleMediaRefresh(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/media?refresh=1", nil)
	w := httptest.NewRecorder()

	h.HandleMedia(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleMediaWithLimit(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/media?limit=1", nil)
	w := httptest.NewRecorder()

	h.HandleMedia(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleMediaETag(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	w := httptest.NewRecorder()

	h.HandleMedia(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	etag := w.Header().Get("ETag")
	if etag != "" {
		req2 := httptest.NewRequest(http.MethodGet, "/api/media", nil)
		req2.Header.Set("If-None-Match", etag)
		w2 := httptest.NewRecorder()

		h.HandleMedia(w2, req2)

		if w2.Code != http.StatusNotModified {
			t.Errorf("expected 304 with matching ETag, got %d", w2.Code)
		}
	}
}

func TestWriteNotModifiedIfMatch(t *testing.T) {
	t.Run("empty etag returns false", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if writeNotModifiedIfMatch(w, req, "", false) {
			t.Error("expected false for empty etag")
		}
	})

	t.Run("refresh returns false", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", "W/test")
		if writeNotModifiedIfMatch(w, req, "W/test", true) {
			t.Error("expected false for refresh=true")
		}
	})

	t.Run("matching etag returns 304", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", "W/test")
		if !writeNotModifiedIfMatch(w, req, "W/test", false) {
			t.Error("expected true for matching etag")
		}
		if w.Code != http.StatusNotModified {
			t.Errorf("expected 304, got %d", w.Code)
		}
	})

	t.Run("non-matching etag returns false", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", "W/other")
		if writeNotModifiedIfMatch(w, req, "W/test", false) {
			t.Error("expected false for non-matching etag")
		}
	})
}

func TestParseLimitParam(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"empty", "", 0},
		{"valid", "limit=5", 5},
		{"invalid", "limit=abc", 0},
		{"negative", "limit=-1", 0},
		{"zero", "limit=0", 0},
		{"large", "limit=1000", 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)
			if got := parseLimitParam(req); got != tt.want {
				t.Errorf("parseLimitParam() = %d, want %d", got, tt.want)
			}
		})
	}
}
