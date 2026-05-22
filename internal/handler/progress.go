package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"msp/internal/constants"
	"msp/internal/domain"
	"msp/internal/service"
)

var validLogLevels = map[string]bool{"debug": true, "info": true, "warning": true, "error": true}

func (h *Handler) HandlePrefs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		prefs, err := h.prefs.GetAllPrefs(r.Context())
		if err != nil {
			h.logger.Log(service.LogLevelError, fmt.Sprintf("Error in GetAllPrefs: %v", err))
			writeJSON(w, http.StatusInternalServerError, domain.PrefsResponse{Error: &domain.ApiError{Message: constants.ErrMsgReadPrefs}})
			return
		}
		writeJSON(w, http.StatusOK, domain.PrefsResponse{Prefs: prefs})
	case http.MethodPost:
		var req domain.PrefsUpdateRequest
		if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
			writeJSONDecodeError(w, err)
			return
		}
		if len(req.Prefs) == 0 {
			writeJSON(w, http.StatusBadRequest, domain.PrefsResponse{Error: &domain.ApiError{Message: constants.ErrMsgMissingPrefs}})
			return
		}
		if err := h.prefs.SetPrefs(r.Context(), req.Prefs); err != nil {
			h.logger.Log(service.LogLevelError, fmt.Sprintf("Error in SetPrefs: %v", err))
			writeJSON(w, http.StatusInternalServerError, domain.PrefsResponse{Error: &domain.ApiError{Message: constants.ErrMsgWritePrefs}})
			return
		}
		writeJSON(w, http.StatusOK, domain.PrefsResponse{Prefs: req.Prefs})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleProgress(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, constants.ErrMsgMissingID)
			return
		}
		t, err := h.progress.GetProgress(r.Context(), id)
		if err != nil {
			h.logger.Log(service.LogLevelError, fmt.Sprintf("Error in GetProgress: %v", err))
			writeError(w, http.StatusInternalServerError, constants.ErrMsgReadProgress)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"time": t})
	case http.MethodPost:
		var req struct {
			ID   string  `json:"id"`
			Time float64 `json:"time"`
		}
		if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
			writeJSONDecodeError(w, err)
			return
		}
		if req.ID == "" {
			writeError(w, http.StatusBadRequest, constants.ErrMsgMissingID)
			return
		}
		if err := h.progress.SetProgress(r.Context(), req.ID, req.Time); err != nil {
			h.logger.Log(service.LogLevelError, fmt.Sprintf("Error in SetProgress: %v", err))
			writeError(w, http.StatusInternalServerError, constants.ErrMsgWriteProgress)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req domain.LogRequest
	if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
		writeJSONDecodeError(w, err)
		return
	}
	if req.Level != "" && !validLogLevels[req.Level] {
		writeError(w, http.StatusBadRequest, "invalid log level")
		return
	}
	if req.Msg != "" {
		if len(req.Msg) > 500 {
			req.Msg = req.Msg[:500]
		}
		req.Msg = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(req.Msg)
		h.logger.Log(req.Level, req.Msg)
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleRecentProgress returns recently played media with progress.
// GET /api/progress/recent?limit=10
func (h *Handler) HandleRecentProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 50 {
		limit = v
	}
	items, err := h.progress.ListRecentProgress(r.Context(), limit)
	if err != nil {
		h.logger.Log(service.LogLevelError, fmt.Sprintf("Error in ListRecentProgress: %v", err))
		writeError(w, http.StatusInternalServerError, "failed to list recent progress")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
