package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"msp/internal/util"
)

func TestHandleSubtitleMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/subtitle", nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleSubtitleMissingID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle", nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSubtitleBadID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id=!!!invalid!!!", nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSubtitleFileNotFound(t *testing.T) {
	h, _ := setupTestHandler(t)

	tmpDir := t.TempDir()
	vttPath := filepath.Join(tmpDir, "nonexistent.vtt")
	id := util.EncodeID(vttPath)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 (file not in shares), got %d", w.Code)
	}
}

func TestHandleSubtitleForbiddenFile(t *testing.T) {
	h, _ := setupTestHandler(t)

	tmpDir := t.TempDir()
	vttPath := filepath.Join(tmpDir, "test.vtt")
	_ = os.WriteFile(vttPath, []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n"), 0600)
	id := util.EncodeID(vttPath)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for file not in shares, got %d", w.Code)
	}
}

func TestHandleSubtitleVTT(t *testing.T) {
	tmpDir := t.TempDir()
	s := newTestServerWithShare(t, tmpDir)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: nil, Prefs: nil})

	vttContent := "WEBVTT\n\n00:00:01.000 --> 00:00:05.000\nHello World\n"
	vttPath := filepath.Join(tmpDir, "test.vtt")
	_ = os.WriteFile(vttPath, []byte(vttContent), 0600)
	id := util.EncodeID(vttPath)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/vtt; charset=utf-8" {
		t.Errorf("expected text/vtt content type, got %s", ct)
	}
}

func TestHandleSubtitleSRT(t *testing.T) {
	tmpDir := t.TempDir()
	s := newTestServerWithShare(t, tmpDir)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: nil, Prefs: nil})

	srtContent := "1\n00:00:01,000 --> 00:00:05,000\nHello World\n"
	srtPath := filepath.Join(tmpDir, "test.srt")
	_ = os.WriteFile(srtPath, []byte(srtContent), 0600)
	id := util.EncodeID(srtPath)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/vtt; charset=utf-8" {
		t.Errorf("expected text/vtt for converted srt, got %s", ct)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "WEBVTT") {
		t.Error("SRT should be converted to VTT format")
	}
}

func TestHandleSubtitleASS(t *testing.T) {
	tmpDir := t.TempDir()
	s := newTestServerWithShare(t, tmpDir)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: nil, Prefs: nil})

	assContent := `[Script Info]
Title: Test
[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Hello World
`
	assPath := filepath.Join(tmpDir, "test.ass")
	_ = os.WriteFile(assPath, []byte(assContent), 0600)
	id := util.EncodeID(assPath)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/vtt; charset=utf-8" {
		t.Errorf("expected text/vtt for converted ass, got %s", ct)
	}
}

func TestHandleSubtitleUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	s := newTestServerWithShare(t, tmpDir)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: nil, Prefs: nil})

	txtPath := filepath.Join(tmpDir, "test.xyz")
	_ = os.WriteFile(txtPath, []byte("not a subtitle"), 0600)
	id := util.EncodeID(txtPath)

	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unsupported format, got %d", w.Code)
	}
}
