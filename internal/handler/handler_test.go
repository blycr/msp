package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerNew(t *testing.T) {
	h := New(Deps{})
	if h == nil {
		t.Fatal("Expected New handler to not be nil")
	}
}

func TestHandleIP(t *testing.T) {
	h := New(Deps{})
	req := httptest.NewRequest("GET", "/api/ip", nil)
	w := httptest.NewRecorder()
	h.HandleIP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestDetermineContentTypeWMV(t *testing.T) {
	ct := determineContentType(".wmv")
	if ct != "video/x-ms-wmv" {
		t.Fatalf("expected video/x-ms-wmv, got %s", ct)
	}
}