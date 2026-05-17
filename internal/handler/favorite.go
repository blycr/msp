package handler

import (
	"net/http"
)

// HandleFavorites handles GET/POST/DELETE /api/favorites
func (h *Handler) HandleFavorites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.favorites.ListFavorites(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list favorites")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})

	case http.MethodPost:
		var req struct {
			MediaID string `json:"mediaId"`
		}
		if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
			writeJSONDecodeError(w, err)
			return
		}
		if req.MediaID == "" {
			writeError(w, http.StatusBadRequest, "missing mediaId")
			return
		}
		if err := h.favorites.AddFavorite(r.Context(), req.MediaID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add favorite")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing id")
			return
		}
		if err := h.favorites.RemoveFavorite(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to remove favorite")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
