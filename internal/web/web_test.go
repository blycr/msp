package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestServeEmbeddedFile(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("hello")},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	ServeEmbeddedWeb(w, req, fs)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}
