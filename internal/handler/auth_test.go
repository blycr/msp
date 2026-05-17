package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"msp/internal/domain"
	"msp/internal/server"
	"msp/internal/storage"
)

// setupTestHandlerWithRealServer 创建一个具有真实 server 和临时目录 share 的 Handler
func setupTestHandlerWithRealServer(t *testing.T) (*Handler, *server.Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	s := server.New(cfgPath, nil)
	store := storage.NewStore(nil)
	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: store, Prefs: store, Favorites: store})
	return h, s, tmpDir
}

func TestHandleShares_MethodNotAllowed(t *testing.T) {
	h, _, _ := setupTestHandlerWithRealServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/shares", nil)
	w := httptest.NewRecorder()
	h.HandleShares(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleShares_InvalidJSON(t *testing.T) {
	h, _, _ := setupTestHandlerWithRealServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/shares", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	h.HandleShares(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleShares_AddNonExistentDir(t *testing.T) {
	h, _, _ := setupTestHandlerWithRealServer(t)
	body, _ := json.Marshal(domain.SharesOpRequest{
		Op:   "add",
		Path: "/path/that/does/not/exist",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/shares", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleShares(w, req)
	// handleShareAdd 对"目录不存在"返回 error，不含 "exists"/"missing" 英文词，
	// 所以 HandleShares 走 500 分支（这是当前实现的行为）
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-existent dir, got %d", w.Code)
	}
}

func TestHandleShares_AddAndRemoveShare(t *testing.T) {
	h, s, tmpDir := setupTestHandlerWithRealServer(t)

	shareDir := filepath.Join(tmpDir, "mymovies")
	if err := os.MkdirAll(shareDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// 添加 share
	addBody, _ := json.Marshal(domain.SharesOpRequest{
		Op:    "add",
		Path:  shareDir,
		Label: "My Movies",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/shares", bytes.NewReader(addBody))
	w := httptest.NewRecorder()
	h.HandleShares(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("add share: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// 检查 share 已加入
	found := false
	for _, sh := range s.Config().Shares {
		if sh.Label == "My Movies" {
			found = true
			break
		}
	}
	if !found {
		t.Error("share 'My Movies' not found after add")
	}

	// 删除 share
	rmBody, _ := json.Marshal(domain.SharesOpRequest{
		Op:   "remove",
		Path: shareDir,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/shares", bytes.NewReader(rmBody))
	w2 := httptest.NewRecorder()
	h.HandleShares(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("remove share: expected 200, got %d", w2.Code)
	}

	// 检查 share 已移除
	for _, sh := range s.Config().Shares {
		if sh.Label == "My Movies" {
			t.Error("share 'My Movies' should be removed")
		}
	}
}

func TestHandleShares_UnsupportedOp(t *testing.T) {
	h, _, _ := setupTestHandlerWithRealServer(t)
	body, _ := json.Marshal(domain.SharesOpRequest{Op: "rename", Path: "/some/path"})
	req := httptest.NewRequest(http.MethodPost, "/api/shares", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleShares(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unsupported op, got %d", w.Code)
	}
}


