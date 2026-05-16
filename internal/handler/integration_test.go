package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/media"
	"msp/internal/scanner"
	"msp/internal/server"
	"msp/internal/storage"
	"msp/internal/util"
)

func setupIntegrationTest(t *testing.T) (*Handler, *server.Server, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	sq, err := storage.InitSQLite(dbPath)
	if err != nil {
		t.Fatalf("InitSQLite: %v", err)
	}

	idCodec := util.NewIDCodec(nil)
	processor := media.NewMediaProcessor(sq, idCodec)

	cfgPath := filepath.Join(tmpDir, "config.json")
	s := server.New(cfgPath, processor)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig: %v", err)
	}

	h := New(Deps{
		Config:    s,
		Media:     s.MediaSvc,
		Session:   s,
		Logger:    s,
		Progress:  storage.NewStore(sq),
		Prefs:     storage.NewStore(sq),
		Processor: processor,
		IDCodec:   idCodec,
	})

	cleanup := func() {
		sq.Close()
	}
	return h, s, cleanup
}

func TestScanThenList(t *testing.T) {
	tmpDir := t.TempDir()
	videoFile := filepath.Join(tmpDir, "movie.mp4")
	if err := os.WriteFile(videoFile, []byte("dummy"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	idCodec := util.NewIDCodec(nil)
	shares := []domain.Share{{Label: "Media", Path: tmpDir}}
	resp, err := media.BuildMediaResponse(context.Background(), shares, config.BlacklistConfig{}, 0, idCodec)
	if err != nil {
		t.Fatalf("BuildMediaResponse: %v", err)
	}

	if len(resp.Videos) != 1 {
		t.Fatalf("expected 1 video, got %d", len(resp.Videos))
	}
	if resp.Videos[0].Name != "movie.mp4" {
		t.Errorf("expected movie.mp4, got %s", resp.Videos[0].Name)
	}
}

func TestMediaIDStability(t *testing.T) {
	idCodec := util.NewIDCodec(make([]byte, 32))
	path := "/path/to/file.mp4"
	id1 := idCodec.EncodeID(path)
	id2 := idCodec.EncodeID(path)
	if id1 != id2 {
		t.Errorf("MediaID not deterministic: %q vs %q", id1, id2)
	}
}

func TestMediaIDStabilityViaScanner(t *testing.T) {
	tmpDir := t.TempDir()
	videoFile := filepath.Join(tmpDir, "movie.mp4")
	if err := os.WriteFile(videoFile, []byte("dummy"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	idCodec := util.NewIDCodec(make([]byte, 32))
	shares := []domain.Share{{Label: "Media", Path: tmpDir}}
	var ids []string

	for i := 0; i < 3; i++ {
		resp, err := media.BuildMediaResponse(context.Background(), shares, config.BlacklistConfig{}, 0, idCodec)
		if err != nil {
			t.Fatalf("BuildMediaResponse: %v", err)
		}
		if len(resp.Videos) != 1 {
			t.Fatalf("expected 1 video, got %d", len(resp.Videos))
		}
		ids = append(ids, resp.Videos[0].ID)
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] != ids[0] {
			t.Errorf("ID changed across scans: run 0=%q run %d=%q", ids[0], i, ids[i])
		}
	}
}

func TestHandleMediaIntegration(t *testing.T) {
	h, _, cleanup := setupIntegrationTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/media", nil)
	w := httptest.NewRecorder()
	h.HandleMedia(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp domain.MediaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestStreamRange(t *testing.T) {
	tmpDir := t.TempDir()
	videoFile := filepath.Join(tmpDir, "movie.mp4")
	if err := os.WriteFile(videoFile, []byte("0123456789"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	sq, err := storage.InitSQLite(dbPath)
	if err != nil {
		t.Fatalf("InitSQLite: %v", err)
	}
	defer sq.Close()

	idCodec := util.NewIDCodec(nil)
	processor := media.NewMediaProcessor(sq, idCodec)

	cfgPath := filepath.Join(tmpDir, "config.json")
	s := server.New(cfgPath, processor)
	if err := s.LoadOrInitConfig(); err != nil {
		t.Fatalf("LoadOrInitConfig: %v", err)
	}

	cfg := s.Config()
	cfg.Shares = []domain.Share{{Label: "Media", Path: tmpDir}}
	_ = s.UpdateConfig(func(c *config.Config) {
		c.Shares = cfg.Shares
	})

	h := New(Deps{
		Config:    s,
		Media:     s.MediaSvc,
		Session:   s,
		Logger:    s,
		Progress:  storage.NewStore(sq),
		Prefs:     storage.NewStore(sq),
		Processor: processor,
		IDCodec:   idCodec,
	})

	videoID := idCodec.EncodeID(videoFile)
	req := httptest.NewRequest("GET", "/api/stream?id="+videoID, nil)
	req.Header.Set("Range", "bytes=0-4")
	w := httptest.NewRecorder()
	h.HandleStream(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected 206 PartialContent, got %d", w.Code)
	}
}

func TestSubtitleIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	videoFile := filepath.Join(tmpDir, "movie.mp4")
	vttFile := filepath.Join(tmpDir, "movie.vtt")
	if err := os.WriteFile(videoFile, []byte("dummy"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(vttFile, []byte("WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	idCodec := util.NewIDCodec(nil)
	subs := scanner.FindSidecarSubtitles(videoFile, idCodec)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subtitle, got %d", len(subs))
	}
	if subs[0].Label != "字幕" {
		t.Errorf("expected 字幕, got %s", subs[0].Label)
	}
}
