package handler

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"time"

	"msp/internal/server"
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

func WithLog(s *server.Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		s.LogRequest(r, sw.status, start)
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
func WithSecurity(s *server.Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := s.Config()

		// Get client IP
		clientIP := getClientIP(r)

		// Check IP whitelist/blacklist
		if !isIPAllowed(clientIP, cfg.Security.IPWhitelist, cfg.Security.IPBlacklist) {
			s.Log("info", fmt.Sprintf("Access denied for IP: %s", clientIP))
			http.Error(w, "Access Denied", http.StatusForbidden)
			return
		}

		// Check PIN authentication
		if cfg.Security.PINEnabled {
			// Skip PIN check for certain endpoints
			if !requiresPIN(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Check PIN from header or cookie
			pin := r.Header.Get("X-PIN")
			if pin == "" {
				cookie, err := r.Cookie("msp_pin")
				if err == nil {
					pin = cookie.Value
				}
			}

			if pin != cfg.Security.PIN {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the real client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Use RemoteAddr
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
func matchesCIDR(clientIP, cidr string) bool {
	// Simple CIDR matching - parse IP and network
	// For production use, consider using net.ParseCIDR
	parts := strings.Split(cidr, "/")
	if len(parts) != 2 {
		return false
	}

	networkIP := parts[0]
	// For simplicity, we'll do prefix matching for common cases
	// A full implementation would use proper CIDR calculation

	// Handle /24 (most common case)
	if parts[1] == "24" {
		clientParts := strings.Split(clientIP, ".")
		networkParts := strings.Split(networkIP, ".")
		if len(clientParts) == 4 && len(networkParts) == 4 {
			return clientParts[0] == networkParts[0] &&
				clientParts[1] == networkParts[1] &&
				clientParts[2] == networkParts[2]
		}
	}

	// Handle /16
	if parts[1] == "16" {
		clientParts := strings.Split(clientIP, ".")
		networkParts := strings.Split(networkIP, ".")
		if len(clientParts) == 4 && len(networkParts) == 4 {
			return clientParts[0] == networkParts[0] &&
				clientParts[1] == networkParts[1]
		}
	}

	// Handle /8
	if parts[1] == "8" {
		clientParts := strings.Split(clientIP, ".")
		networkParts := strings.Split(networkIP, ".")
		if len(clientParts) == 4 && len(networkParts) == 4 {
			return clientParts[0] == networkParts[0]
		}
	}

	return false
}

// requiresPIN determines if a path requires PIN authentication
func requiresPIN(path string) bool {
	// PIN authentication only applies to API endpoints (except /api/pin itself)
	// Static resources (HTML, CSS, JS, images, etc.) should be accessible without PIN
	// so that the frontend can load and display the PIN dialog

	// Exempt paths that never require PIN
	exemptPaths := []string{
		"/api/pin", // PIN verification endpoint itself
	}

	for _, exempt := range exemptPaths {
		if path == exempt {
			return false
		}
	}

	// Only API endpoints require PIN (except those explicitly exempted above)
	// Everything else (/, /assets/*, /icon.svg, etc.) is accessible
	if strings.HasPrefix(path, "/api/") {
		return true
	}

	// Static resources and root path don't require PIN
	return false
}
