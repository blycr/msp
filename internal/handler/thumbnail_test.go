package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
