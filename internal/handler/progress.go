package handler

import (
	"log"
	"net/http"

	"msp/internal/constants"
	"msp/internal/domain"
)

func (h *Handler) HandlePrefs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		prefs, err := h.prefs.GetAllPrefs(r.Context())
		if err != nil {
			log.Printf("Error in GetAllPrefs: %v", err)
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
			log.Printf("Error in SetPrefs: %v", err)
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
			log.Printf("Error in GetProgress: %v", err)
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
			log.Printf("Error in SetProgress: %v", err)
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
	if req.Msg != "" {
		h.logger.Log(req.Level, req.Msg)
	}
	w.WriteHeader(http.StatusNoContent)
}
