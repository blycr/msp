package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"msp/internal/server"
	"msp/internal/storage"
)

func setupFavoriteHandler(t *testing.T) *Handler {
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

func TestHandleFavorites_Get(t *testing.T) {
	h := setupFavoriteHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/favorites", nil)
	w := httptest.NewRecorder()

	h.HandleFavorites(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("expected items array in response")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestHandleFavorites_PostAndGet(t *testing.T) {
	h := setupFavoriteHandler(t)

	// Add favorite
	body := `{"mediaId":"test-media-id"}`
	reqPost := httptest.NewRequest(http.MethodPost, "/api/favorites", strings.NewReader(body))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	h.HandleFavorites(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("expected 200 on post, got %d", wPost.Code)
	}

	// List favorites
	reqGet := httptest.NewRequest(http.MethodGet, "/api/favorites", nil)
	wGet := httptest.NewRecorder()
	h.HandleFavorites(wGet, reqGet)

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

func TestHandleFavorites_PostMissingID(t *testing.T) {
	h := setupFavoriteHandler(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/favorites", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleFavorites(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing mediaId, got %d", w.Code)
	}
}

func TestHandleFavorites_Delete(t *testing.T) {
	h := setupFavoriteHandler(t)

	// Add first
	body := `{"mediaId":"del-id"}`
	reqPost := httptest.NewRequest(http.MethodPost, "/api/favorites", strings.NewReader(body))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	h.HandleFavorites(wPost, reqPost)

	// Delete
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/favorites?id=del-id", nil)
	wDel := httptest.NewRecorder()
	h.HandleFavorites(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Errorf("expected 200 on delete, got %d", wDel.Code)
	}

	// Verify empty
	reqGet := httptest.NewRequest(http.MethodGet, "/api/favorites", nil)
	wGet := httptest.NewRecorder()
	h.HandleFavorites(wGet, reqGet)

	var resp map[string]any
	if err := json.NewDecoder(wGet.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	items, _ := resp["items"].([]any)
	if len(items) != 0 {
		t.Errorf("expected 0 items after delete, got %d", len(items))
	}
}

func TestHandleFavorites_InvalidMethod(t *testing.T) {
	h := setupFavoriteHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/api/favorites", nil)
	w := httptest.NewRecorder()
	h.HandleFavorites(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
