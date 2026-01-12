package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerNew(t *testing.T) {
	h := New(nil)
	if h == nil {
		t.Fatal("Expected New handler to not be nil")
	}
}

func TestHandleIP(t *testing.T) {
	h := New(nil)
	req := httptest.NewRequest("GET", "/api/ip", nil)
	w := httptest.NewRecorder()
	h.HandleIP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}
