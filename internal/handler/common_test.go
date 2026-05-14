package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeJSON(w, http.StatusOK, map[string]string{"key": "value"})
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Errorf("expected json content-type, got %s", ct)
		}
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if body["key"] != "value" {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})

	t.Run("marshal_error_fallback", func(t *testing.T) {
		w := httptest.NewRecorder()
		// channel cannot be marshaled to JSON
		writeJSON(w, http.StatusOK, make(chan int))
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 on marshal error, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "error") {
			t.Errorf("expected error body, got %s", w.Body.String())
		}
	})
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "bad request")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", resp["error"])
	}
	if errObj["message"] != "bad request" {
		t.Errorf("unexpected message: %v", errObj["message"])
	}
}

func TestDecodeJSONBody(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"test"}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		var dst struct{ Name string }
		if err := decodeJSONBody(nil, r, &dst, 1024); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dst.Name != "test" {
			t.Errorf("unexpected name: %s", dst.Name)
		}
	})

	t.Run("payload_too_large", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"test"}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		var dst struct{ Name string }
		w := httptest.NewRecorder()
		err := decodeJSONBody(w, r, &dst, 5)
		if err == nil {
			t.Fatal("expected error for large payload")
		}
		if !isPayloadTooLarge(err) {
			t.Errorf("expected MaxBytesError, got %T", err)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		var dst struct{ Name string }
		if err := decodeJSONBody(nil, r, &dst, 1024); err == nil {
			t.Fatal("expected error for invalid json")
		}
	})
}

func TestIsPayloadTooLarge(t *testing.T) {
	if isPayloadTooLarge(nil) {
		t.Error("nil should not be payload too large")
	}
	if isPayloadTooLarge(errors.New("random error")) {
		t.Error("random error should not be payload too large")
	}
}

func TestWriteJSONDecodeError(t *testing.T) {
	t.Run("payload_too_large", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := bytes.NewBufferString(`{"x":1}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		var dst struct{ X int }
		err := decodeJSONBody(w, r, &dst, 5)
		writeJSONDecodeError(w, err)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected 413, got %d", w.Code)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeJSONDecodeError(w, errors.New("random error"))
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}
