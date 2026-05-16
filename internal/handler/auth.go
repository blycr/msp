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
	pinAttemptsMu    sync.RWMutex
	maxPINFailures   = 5
	pinBlockDuration = 15 * time.Minute
	maxPinAttempts   = 1000
)

func getPINAttempt(ip string) *pinAttempt {
	pinAttemptsMu.Lock()
	defer pinAttemptsMu.Unlock()

	// Evict a random entry if at capacity and this is a new IP.
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
	return entry
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
	attempt := getPINAttempt(clientIP)

	if attempt.blockedUntil.After(time.Now()) {
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
		attempt.failures++
		attempt.lastFailure = time.Now()
		if attempt.failures >= maxPINFailures {
			attempt.blockedUntil = time.Now().Add(pinBlockDuration)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":   valid,
		"enabled": true,
	})
}


