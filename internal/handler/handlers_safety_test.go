package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/server"
	"msp/internal/storage"
)

func TestApplyLimitOnlyWhenTruncated(t *testing.T) {
	resp := domain.MediaResponse{
		Videos: []domain.MediaItem{{Name: "v1"}},
		Audios: []domain.MediaItem{{Name: "a1"}},
	}
	if applyLimit(&resp, 10) {
		t.Fatal("applyLimit should be false when nothing is truncated")
	}
	if resp.Limited {
		t.Fatal("resp.Limited should be false when nothing is truncated")
	}

	resp2 := domain.MediaResponse{
		Videos: []domain.MediaItem{{Name: "v1"}, {Name: "v2"}},
	}
	if !applyLimit(&resp2, 1) {
		t.Fatal("applyLimit should be true when truncation happens")
	}
	if !resp2.Limited {
		t.Fatal("resp2.Limited should be true when truncation happens")
	}
	if len(resp2.Videos) != 1 {
		t.Fatalf("expected truncated videos length 1, got %d", len(resp2.Videos))
	}
}

func TestHandlePINRejectsLargePayload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	s := server.New(configPath, nil)
	if err := s.UpdateConfig(func(cfg *config.Config) {
		cfg.Security.PINEnabled = true
		cfg.Security.PIN = "1234"
	}); err != nil {
		t.Fatalf("failed to setup server config: %v", err)
	}

	store := storage.NewStore(nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store})
	tooLargePIN := strings.Repeat("1", int(defaultJSONBodyLimit)+1)
	body := `{"pin":"` + tooLargePIN + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/pin", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandlePIN(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", w.Code)
	}
}

func TestHandlePINCookieSecureAlwaysDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	s := server.New(configPath, nil)
	if err := s.UpdateConfig(func(cfg *config.Config) {
		cfg.Security.PINEnabled = true
		cfg.Security.PIN = "1234"
	}); err != nil {
		t.Fatalf("failed to setup server config: %v", err)
	}

	store := storage.NewStore(nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store})
	req := httptest.NewRequest(http.MethodPost, "/api/pin", strings.NewReader(`{"pin":"1234"}`))
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	h.HandlePIN(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	found := false
	for _, c := range resp.Cookies() {
		if c.Name == "msp_session" {
			found = true
			if c.Secure {
				t.Fatal("home mode should not set secure cookie")
			}
		}
	}
	if !found {
		t.Fatal("session cookie not found")
	}
}

func TestServeSRTRejectsLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "large.srt")

	// #nosec G304 -- test path is created under t.TempDir().
	fw, err := os.Create(p)
	if err != nil {
		t.Fatalf("failed to create temp subtitle file: %v", err)
	}
	if err := fw.Truncate(maxSubtitleConvertSize + 1); err != nil {
		_ = fw.Close()
		t.Fatalf("failed to grow subtitle file: %v", err)
	}
	_ = fw.Close()

	// #nosec G304 -- test path is created under t.TempDir().
	f, err := os.Open(p)
	if err != nil {
		t.Fatalf("failed to open subtitle file: %v", err)
	}
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		t.Fatalf("failed to stat subtitle file: %v", err)
	}

	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/subtitle", nil)
	w := httptest.NewRecorder()
	h.serveSRT(w, req, f, st)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", w.Code)
	}
}