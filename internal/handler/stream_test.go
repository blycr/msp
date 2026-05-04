package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/util"
)

func TestDetermineContentType(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".mp4", "video/mp4"},
		{".mkv", "video/x-matroska"},
		{".webm", "video/webm"},
		{".avi", "video/x-msvideo"},
		{".wmv", "video/x-ms-wmv"},
		{".mov", "video/quicktime"},
		{".ts", "video/mp2t"},
		{".vtt", "text/vtt; charset=utf-8"},
		{".srt", "text/plain; charset=utf-8"},
		{".lrc", "text/plain; charset=utf-8"},
		{".m4v", "video/mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := determineContentType(tt.ext)
			if got != tt.want {
				t.Errorf("determineContentType(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestHandleProbeMissingID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/probe", nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleProbeBadID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/probe?id=!!!invalid!!!", nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleProbeForbiddenFile(t *testing.T) {
	h, _ := setupTestHandler(t)

	target := filepath.Join(t.TempDir(), "test.mp4")
	_ = os.WriteFile(target, []byte("fake"), 0644)
	id := util.EncodeID(target)

	req := httptest.NewRequest(http.MethodGet, "/api/probe?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for file not in shares, got %d", w.Code)
	}
}

func TestHandleStreamMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/stream", nil)
	w := httptest.NewRecorder()

	h.HandleStream(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleStreamMissingID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
	w := httptest.NewRecorder()

	h.HandleStream(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleProbeMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/probe", nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleProbeWithValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	s := newTestServerWithShare(t, configPath, tmpDir)

	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: nil, Prefs: nil})

	videoPath := filepath.Join(tmpDir, "test.mp4")
	_ = os.WriteFile(videoPath, []byte("fake-mp4-data"), 0644)
	id := util.EncodeID(videoPath)

	req := httptest.NewRequest(http.MethodGet, "/api/probe?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleStreamForbiddenFile(t *testing.T) {
	h, _ := setupTestHandler(t)

	target := filepath.Join(t.TempDir(), "test.mp4")
	_ = os.WriteFile(target, []byte("fake"), 0644)
	id := util.EncodeID(target)

	req := httptest.NewRequest(http.MethodGet, "/api/stream?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleStream(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for file not in shares, got %d", w.Code)
	}
}

func newTestServerWithShare(t *testing.T, configPath, shareDir string) *testServerWrapper {
	t.Helper()
	s := &testServerWrapper{cfg: config.Default()}
	s.cfg.Shares = []domain.Share{{Label: "Test", Path: shareDir}}
	return s
}

type testServerWrapper struct {
	cfg config.Config
}

func (s *testServerWrapper) Config() config.Config {
	return s.cfg
}

func (s *testServerWrapper) UpdateConfig(fn func(*config.Config)) error {
	fn(&s.cfg)
	return nil
}

func (s *testServerWrapper) GetPort() int {
	return s.cfg.Port
}

func (s *testServerWrapper) GetOrBuildMediaCache(_ context.Context, _ []domain.Share, _ config.BlacklistConfig, _ bool) (domain.MediaResponse, string) {
	return domain.MediaResponse{}, ""
}

func (s *testServerWrapper) InvalidateMediaCache() {}

func (s *testServerWrapper) CreateSession() (string, error) {
	return "test-session", nil
}

func (s *testServerWrapper) ValidateSession(_ string) bool {
	return true
}

func (s *testServerWrapper) Log(_ string, _ string) {}

func (s *testServerWrapper) LogRequest(_ *http.Request, _ int, _ time.Time) {}
