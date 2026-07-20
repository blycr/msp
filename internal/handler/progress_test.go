package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"msp/internal/server"
	"msp/internal/storage"
)

func setupProgressHandler(t *testing.T) *Handler {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	s := server.New(configPath, nil)
	sq, err := storage.InitSQLite(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to init sqlite: %v", err)
	}
	t.Cleanup(func() { sq.Close() })
	store := storage.NewStore(sq)
	return New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store, Favorites: store})
}

func TestHandleProgressGetNoID(t *testing.T) {
	h := setupProgressHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/progress", nil)
	w := httptest.NewRecorder()

	h.HandleProgress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleProgressGetEmpty(t *testing.T) {
	h := setupProgressHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/progress?id=test-id", nil)
	w := httptest.NewRecorder()

	h.HandleProgress(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if tm, ok := body["time"].(float64); !ok || tm != 0 {
		t.Errorf("expected time 0, got %v", body["time"])
	}
}

func TestHandleProgressPostAndGet(t *testing.T) {
	h := setupProgressHandler(t)

	body := `{"id":"test-id","time":42.5}`
	req := httptest.NewRequest(http.MethodPost, "/api/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleProgress(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/progress?id=test-id", nil)
	w2 := httptest.NewRecorder()

	h.HandleProgress(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if tm, ok := resp["time"].(float64); !ok || tm != 42.5 {
		t.Errorf("expected time 42.5, got %v", resp["time"])
	}
}

func TestHandleProgressInvalidJSON(t *testing.T) {
	h := setupProgressHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/progress", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleProgress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandleRecentProgress(t *testing.T) {
	h := setupProgressHandler(t)

	// Post some progress
	body := `{"id":"test-id-1","time":10.5}`
	reqPost := httptest.NewRequest(http.MethodPost, "/api/progress", strings.NewReader(body))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	h.HandleProgress(wPost, reqPost)

	if wPost.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on post, got %d", wPost.Code)
	}

	// Get recent progress
	reqGet := httptest.NewRequest(http.MethodGet, "/api/progress/recent?limit=5", nil)
	wGet := httptest.NewRecorder()
	h.HandleRecentProgress(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wGet.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(wGet.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("expected items array in response")
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestHandlePrefsGetEmpty(t *testing.T) {
	h := setupProgressHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/prefs", nil)
	w := httptest.NewRecorder()

	h.HandlePrefs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestHandlePrefsPostAndGet(t *testing.T) {
	h := setupProgressHandler(t)

	body := `{"prefs":{"lang":"zh-CN","theme":"dark"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/prefs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandlePrefs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/prefs", nil)
	w2 := httptest.NewRecorder()

	h.HandlePrefs(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	var resp struct {
		Prefs map[string]string `json:"prefs"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Prefs["lang"] != "zh-CN" {
		t.Errorf("expected lang zh-CN, got %v", resp.Prefs["lang"])
	}
	if resp.Prefs["theme"] != "dark" {
		t.Errorf("expected theme dark, got %v", resp.Prefs["theme"])
	}
}

func TestHandlePrefsInvalidJSON(t *testing.T) {
	h := setupProgressHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/prefs", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandlePrefs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandlePrefsMissingPrefs(t *testing.T) {
	h := setupProgressHandler(t)

	body := `{"prefs":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/prefs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandlePrefs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty prefs, got %d", w.Code)
	}
}

func TestHandleLog(t *testing.T) {
	h := setupProgressHandler(t)

	body := `{"level":"info","msg":"test log message"}`
	req := httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLog(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestHandleLogInvalidJSON(t *testing.T) {
	h := setupProgressHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLog(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandleSharesMethodNotAllowed(t *testing.T) {
	h := setupProgressHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/shares", nil)
	w := httptest.NewRecorder()

	h.HandleShares(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleSharesAdd(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	s := server.New(configPath, nil)
	t.Cleanup(func() {
		s.WaitForBackgroundMediaOps()
	})
	store := storage.NewStore(nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store, Favorites: store})

	shareDir := filepath.Join(tmpDir, "media")
	_ = os.MkdirAll(shareDir, 0750)

	body := `{"op":"add","path":"` + strings.ReplaceAll(shareDir, `\`, `\\`) + `","label":"Media"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleShares(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}
