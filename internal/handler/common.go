package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"msp/internal/constants"
	"msp/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
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