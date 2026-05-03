package handler

import (
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"msp/internal/constants"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	gw *gzip.Writer
}

func (g gzipResponseWriter) Write(p []byte) (int, error) {
	return g.gw.Write(p)
}

func WithGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ae := r.Header.Get("Accept-Encoding")
		if !strings.Contains(ae, "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/stream" || r.URL.Path == "/api/subtitle" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzip.NewWriter(w)
		defer func() { _ = gw.Close() }()
		next.ServeHTTP(gzipResponseWriter{ResponseWriter: w, gw: gw}, r)
	})
}

func WithLog(logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		logger.LogRequest(r, sw.status, start)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// WithSecurity applies IP filtering and PIN authentication
func WithSecurity(config ConfigProvider, session SessionProvider, logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Config()

		// Apply security headers to all responses
		applySecurityHeaders(w)

		// Home/LAN mode: only trust direct TCP peer address.
		clientIP := getClientIP(r, false)

		// Check IP whitelist/blacklist
		if !isIPAllowed(clientIP, cfg.Security.IPWhitelist, cfg.Security.IPBlacklist) {
			logger.Log("info", fmt.Sprintf("Access denied for IP: %s", clientIP))
			http.Error(w, constants.ErrMsgAccessDenied, http.StatusForbidden)
			return
		}

		// Check PIN authentication
		if cfg.Security.PINEnabled {
			// Skip PIN check for certain endpoints
			if !requiresPIN(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Check session token from header or cookie
			sessionToken := r.Header.Get("X-Session-Token")
			if sessionToken == "" {
				cookie, err := r.Cookie("msp_session")
				if err == nil {
					sessionToken = cookie.Value
				}
			}

			if !session.ValidateSession(sessionToken) {
				http.Error(w, constants.ErrMsgUnauthorized, http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// applySecurityHeaders adds security headers to HTTP responses
func applySecurityHeaders(w http.ResponseWriter) {
	// Prevent MIME type sniffing
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Prevent clickjacking
	w.Header().Set("X-Frame-Options", "DENY")
	// Enable XSS protection in browsers
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	// Referrer policy
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

// getClientIP extracts client IP from RemoteAddr only.
// Home/LAN mode does not trust proxy headers.
func getClientIP(r *http.Request, _ bool) string {
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// Remove brackets for IPv6
	ip = strings.Trim(ip, "[]")
	return ip
}

// isIPAllowed checks if an IP is allowed based on whitelist and blacklist
func isIPAllowed(clientIP string, whitelist, blacklist []string) bool {
	// If whitelist is not empty, IP must be in whitelist
	if len(whitelist) > 0 {
		if !matchesIPList(clientIP, whitelist) {
			return false
		}
	}

	// If IP is in blacklist, deny access
	if len(blacklist) > 0 {
		if matchesIPList(clientIP, blacklist) {
			return false
		}
	}

	return true
}

// matchesIPList checks if an IP matches any entry in the list
// Supports both exact IP match and CIDR notation
func matchesIPList(clientIP string, ipList []string) bool {
	for _, entry := range ipList {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Check for CIDR notation
		if strings.Contains(entry, "/") {
			if matchesCIDR(clientIP, entry) {
				return true
			}
		} else {
			// Exact IP match
			if clientIP == entry {
				return true
			}
		}
	}
	return false
}

// matchesCIDR checks if an IP matches a CIDR range
// Uses standard library net.ParseCIDR for proper CIDR matching
func matchesCIDR(clientIP, cidr string) bool {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	return ipNet.Contains(ip)
}

// requiresPIN determines if a path requires PIN authentication
func requiresPIN(path string) bool {
	// PIN authentication only applies to API endpoints (except exempted ones)
	// Static resources (HTML, CSS, JS, images, etc.) should be accessible without PIN
	// so that the frontend can load and display the PIN dialog

	// Exempt paths that never require PIN
	exemptPaths := []string{
		"/api/pin",    // PIN verification endpoint itself
		"/api/ip",     // IP info needed for UI
		"/api/config", // Config needed to check if PIN is enabled
	}

	if slices.Contains(exemptPaths, path) {
		return false
	}

	// Only API endpoints require PIN (except those explicitly exempted above)
	// Everything else (/, /assets/*, /icon.svg, etc.) is accessible
	if strings.HasPrefix(path, "/api/") {
		return true
	}

	// Static resources and root path don't require PIN
	return false
}
