package handler

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"msp/internal/constants"
)

// --- PIN brute-force protection ---

type pinAttempt struct {
	failures     int
	lastFailure  time.Time
	blockedUntil time.Time
}

var (
	pinAttempts      = make(map[string]*pinAttempt)
	pinAttemptsMu    sync.Mutex
	maxPINFailures   = 5
	pinBlockDuration = 15 * time.Minute
	maxPinAttempts   = 1000
)

func pinLocked(ip string) bool {
	pinAttemptsMu.Lock()
	defer pinAttemptsMu.Unlock()
	entry, ok := pinAttempts[ip]
	return ok && entry.blockedUntil.After(time.Now())
}

func recordPINFailure(ip string) {
	pinAttemptsMu.Lock()
	defer pinAttemptsMu.Unlock()

	if len(pinAttempts) >= maxPinAttempts {
		if _, exists := pinAttempts[ip]; !exists {
			for k := range pinAttempts {
				delete(pinAttempts, k)
				break
			}
		}
	}

	entry, exists := pinAttempts[ip]
	if !exists {
		entry = &pinAttempt{}
		pinAttempts[ip] = entry
	}
	entry.failures++
	entry.lastFailure = time.Now()
	if entry.failures >= maxPINFailures {
		entry.blockedUntil = time.Now().Add(pinBlockDuration)
	}
}

func resetPINAttempt(ip string) {
	pinAttemptsMu.Lock()
	delete(pinAttempts, ip)
	pinAttemptsMu.Unlock()
}

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

	// Reject if PIN is enabled but no hash is stored.
	if cfg.Security.PINHash == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   false,
			"enabled": true,
			"error":   "PIN not configured",
		})
		return
	}

	clientIP := getClientIP(r, false)
	if pinLocked(clientIP) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"valid":   false,
			"enabled": true,
			"locked":  true,
		})
		return
	}

	var req struct {
		PIN string `json:"pin"`
	}
	if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	valid := bcrypt.CompareHashAndPassword([]byte(cfg.Security.PINHash), []byte(req.PIN)) == nil
	if valid {
		resetPINAttempt(clientIP)
		sessionToken, err := h.session.CreateSession()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create session")
			return
		}

		secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
		//nolint:gosec // G124: Secure flag is intentionally based on request security
		http.SetCookie(w, &http.Cookie{
			Name:     "msp_session",
			Value:    sessionToken,
			Path:     "/",
			MaxAge:   constants.CookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
	} else {
		recordPINFailure(clientIP)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":   valid,
		"enabled": true,
	})
}
