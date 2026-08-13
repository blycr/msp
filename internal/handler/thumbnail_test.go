package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"msp/internal/util"
)

func TestHandleThumbnail_MissingID(t *testing.T) {
	h, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/thumbnail", nil)
	w := httptest.NewRecorder()
	h.HandleThumbnail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleThumbnail_InvalidMethod(t *testing.T) {
	h, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/thumbnail?id=123", nil)
	w := httptest.NewRecorder()
	h.HandleThumbnail(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleThumbnail_InvalidID(t *testing.T) {
	h, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/thumbnail?id=!!!", nil)
	w := httptest.NewRecorder()
	h.HandleThumbnail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleThumbnail_OutsideShare(t *testing.T) {
	h, _, tmpDir := setupTestHandlerWithRealServer(t)
	mediaDir := filepath.Join(tmpDir, "media")
	if err := os.MkdirAll(mediaDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "ok.mp4"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applySharesOp("add", mediaDir, "Media"); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(tmpDir, "secret.mp4")
	if err := os.WriteFile(outside, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	id := util.NewIDCodec(nil).EncodeID(outside)
	req := httptest.NewRequest(http.MethodGet, "/api/thumbnail?id="+id, nil)
	w := httptest.NewRecorder()
	h.HandleThumbnail(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for path outside shares, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestThumbIsFresh(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mp4")
	thumb := filepath.Join(dir, "thumb.jpg")
	//nolint:gosec // test fixture file
	if err := os.WriteFile(src, []byte("media"), 0600); err != nil {
		t.Fatal(err)
	}

	// 缩略图不存在
	if thumbIsFresh(src, thumb) {
		t.Error("expected false when thumb missing")
	}

	// 空缩略图
	//nolint:gosec // test fixture file
	if err := os.WriteFile(thumb, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if thumbIsFresh(src, thumb) {
		t.Error("expected false when thumb empty")
	}

	// 源文件不存在
	if thumbIsFresh(filepath.Join(dir, "missing.mp4"), thumb) {
		t.Error("expected false when source missing")
	}

	//nolint:gosec // test fixture file
	if err := os.WriteFile(thumb, []byte("thumb"), 0600); err != nil {
		t.Fatal(err)
	}

	base := time.Now()
	//nolint:gosec // test fixture files
	if err := os.Chtimes(thumb, base, base); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // test fixture files
	if err := os.Chtimes(src, base.Add(-time.Minute), base.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	// 缩略图新于源文件 → 新鲜
	if !thumbIsFresh(src, thumb) {
		t.Error("expected true when thumb newer than source")
	}

	// 源文件被替换（mtime 新于缩略图）→ stale
	//nolint:gosec // test fixture files
	if err := os.Chtimes(src, base.Add(time.Minute), base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if thumbIsFresh(src, thumb) {
		t.Error("expected false when source replaced (newer than thumb)")
	}

	// 容差范围内（源仅晚 1s）→ 仍视为新鲜
	//nolint:gosec // test fixture files
	if err := os.Chtimes(src, base.Add(time.Second), base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if !thumbIsFresh(src, thumb) {
		t.Error("expected true within thumbFreshTolerance")
	}
}
