package handler

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"msp/internal/media"
	"msp/internal/server"
)

// newTestHandlerWithProcessor returns a Handler with a real (ffmpeg-less)
// MediaProcessor injected, so session-lookup paths can be exercised.
func newTestHandlerWithProcessor(t *testing.T) *Handler {
	t.Helper()
	s := server.New(filepath.Join(t.TempDir(), "config.json"), nil)
	mp := media.NewMediaProcessor(nil, nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: nil, Prefs: nil, Favorites: nil, Processor: mp})
	t.Cleanup(func() { s.WaitForBackgroundMediaOps() })
	return h
}

func TestHandleHLSMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/hls/abc/index.m3u8", nil)
	w := httptest.NewRecorder()

	h.HandleHLS(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleHLSBadPath(t *testing.T) {
	h, _ := setupTestHandler(t)

	for _, path := range []string{
		"/api/hls/",
		"/api/hls/onlysid",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.HandleHLS(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("path %q: expected 400, got %d", path, w.Code)
		}
	}
}

func TestHandleHLSProcessorUnavailable(t *testing.T) {
	h, _ := setupTestHandler(t)

	// processor 为 nil → 503（生产环境始终注入，测试覆盖降级分支）
	req := httptest.NewRequest(http.MethodGet, "/api/hls/someid/seg_00000.ts", nil)
	w := httptest.NewRecorder()

	h.HandleHLS(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleHLSUnknownSession(t *testing.T) {
	h := newTestHandlerWithProcessor(t)

	req := httptest.NewRequest(http.MethodGet, "/api/hls/doesnotexist/index.m3u8", nil)
	w := httptest.NewRecorder()

	h.HandleHLS(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown session, got %d", w.Code)
	}
}
