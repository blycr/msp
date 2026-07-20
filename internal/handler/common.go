package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"msp/internal/constants"
	"msp/internal/domain"
	"msp/internal/storage"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Warn("writeJSON marshal error", "err", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"内部错误"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": &domain.ApiError{Message: msg},
	})
}

// writeDBUnavailable responds 503 when err is storage.ErrUnavailable.
// It returns true if the error was handled, false otherwise.
func writeDBUnavailable(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, storage.ErrUnavailable) {
		return false
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": "db_unavailable",
	})
	return true
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}

func isPayloadTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	if isPayloadTooLarge(err) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error": &domain.ApiError{Message: "payload too large"},
		})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"error": &domain.ApiError{Message: constants.ErrMsgInvalidJSON},
	})
}

