package handler

import (
	"net/http"

	"msp/internal/constants"
)

func (h *Handler) HandlePIN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cfg := h.config.Config()
	if !cfg.Security.PINEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   true,
			"enabled": false,
		})
		return
	}

	var req struct {
		PIN string `json:"pin"`
	}
	if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
		if isPayloadTooLarge(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"valid": false,
				"error": "payload too large",
			})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"valid": false,
				"error": constants.ErrMsgInvalidRequest,
			})
		}
		return
	}

	valid := constantTimeCompare(req.PIN, cfg.Security.PIN)
	if valid {
		sessionToken, err := h.session.CreateSession()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"valid": false,
				"error": "Failed to create session",
			})
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "msp_session",
			Value:    sessionToken,
			Path:     "/",
			MaxAge:   constants.CookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":   valid,
		"enabled": true,
	})
}

func constantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	result := 0
	for i := 0; i < len(a); i++ {
		result |= int(a[i] ^ b[i])
	}
	return result == 0
}