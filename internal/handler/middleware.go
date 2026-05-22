package handler

import (
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
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

func WithRecovery(logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Log("error", fmt.Sprintf("[PANIC] %v\n%s", rec, debug.Stack()))
				writeError(w, http.StatusInternalServerError, "内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
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

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// WithSecurity applies IP filtering and PIN authentication
func WithSecurity(config ConfigProvider, session SessionProvider, logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Config()

		// Apply security headers to all responses
		applySecurityHeaders(w)

		// HSTS for HTTPS requests
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Home/LAN mode: only trust direct TCP peer address.
		clientIP := getClientIP(r, false)

		// For IP allowlist/blocklist, also check CF-Connecting-IP when behind Tunnel
		realIPForFilter := clientIP
		ip := net.ParseIP(clientIP)
		if ip != nil && ip.IsLoopback() {
			if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
				realIPForFilter = cfIP
			}
		}

		// Check IP whitelist/blacklist
		if !isIPAllowed(realIPForFilter, cfg.Security.IPWhitelist, cfg.Security.IPBlacklist) {
			logger.Log("info", fmt.Sprintf("Access denied for IP: %s (filtered as %s)", clientIP, realIPForFilter))
			writeError(w, http.StatusForbidden, constants.ErrMsgAccessDenied)
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
	// Referrer policy
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	// Content Security Policy
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; "+
		"script-src 'self' 'unsafe-inline' https://static.cloudflareinsights.com; "+
		"style-src 'self' 'unsafe-inline'; "+
		"connect-src 'self' https://cdn.plyr.io; "+
		"img-src 'self' blob: https://cdn.plyr.io; "+
		"media-src 'self' blob: https://cdn.plyr.io; "+
		"manifest-src data:; "+
		"frame-ancestors 'none'; "+
		"base-uri 'self';")
}

// --- Rate Limiting ---

type bucket struct {
	tokens     float64
	lastUpdate time.Time
}

// RateLimiter provides per-IP token-bucket rate limiting.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	maxSize int
}

// NewRateLimiter creates a new in-memory rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		maxSize: 10000,
	}
}

// Allow checks if a request from ip is allowed under the given rate/capacity.
func (rl *RateLimiter) Allow(ip string, rate float64, capacity float64) bool {
	if rate <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Evict a random bucket if at capacity and this is a new IP.
	if len(rl.buckets) >= rl.maxSize {
		if _, exists := rl.buckets[ip]; !exists {
			for k := range rl.buckets {
				delete(rl.buckets, k)
				break
			}
		}
	}

	b, exists := rl.buckets[ip]
	if !exists {
		b = &bucket{tokens: capacity, lastUpdate: time.Now()}
		rl.buckets[ip] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.tokens = minFloat64(capacity, b.tokens+elapsed*rate)
	b.lastUpdate = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func getRateLimitConfig(path, method string, isRefresh bool) (rate float64, capacity float64) {
	switch {
	case path == "/api/pin" && method == http.MethodPost:
		return 0.2, 5 // 1 per 5s, burst 5 — brute-force protection
	case path == "/api/media" && isRefresh:
		return 0.033, 1 // 1 per 30s, burst 1 — scan abuse protection
	case path == "/api/config" && method == http.MethodPost:
		return 0.2, 3 // 1 per 5s, burst 3 — config tampering protection
	case path == "/api/shares" && method == http.MethodPost:
		return 0.2, 3 // 1 per 5s, burst 3 — share tampering protection
	default:
		return 0, 0 // no limit for streaming, progress, subtitles, logs, etc.
	}
}

// WithRateLimit applies token-bucket rate limiting per IP.
// Local and LAN access are exempt from rate limiting.
func WithRateLimit(limiter *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		level := getAccessLevelFromRequest(r)
		if level == AccessLocal || level == AccessLAN {
			next.ServeHTTP(w, r)
			return
		}

		isRefresh := r.URL.Query().Get("refresh") == "1"
		rate, capacity := getRateLimitConfig(r.URL.Path, r.Method, isRefresh)
		if rate <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		ip := getClientIP(r, false)
		if !limiter.Allow(ip, rate, capacity) {
			w.Header().Set("Retry-After", "30")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts client IP from RemoteAddr only.
// Home/LAN mode does not trust proxy headers.
func getClientIP(r *http.Request, _ bool) string {
	ip := r.RemoteAddr
	host, _, err := net.SplitHostPort(ip)
	if err == nil {
		return host
	}
	// Fallback for addresses without port (e.g. some test environments)
	return strings.Trim(ip, "[]")
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

// AccessLevel represents the access level of a client.
type AccessLevel int

const (
	AccessLocal  AccessLevel = iota // localhost / 127.0.0.1 / ::1
	AccessLAN                        // private LAN ranges
	AccessRemote                     // anything else (Cloudflare Tunnel, public IP)
)

// getAccessLevel classifies an IP into Local / LAN / Remote.
func getAccessLevel(clientIP string) AccessLevel {
	// Strip IPv6 zone index (e.g. "fe80::1%eth0" -> "fe80::1")
	if idx := strings.Index(clientIP, "%"); idx != -1 {
		clientIP = clientIP[:idx]
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return AccessRemote
	}

	// Loopback: 127.0.0.1, ::1
	if ip.IsLoopback() {
		return AccessLocal
	}

	// IPv4 private ranges
	if ip.To4() != nil {
		if isPrivateIPv4(ip) {
			return AccessLAN
		}
		return AccessRemote
	}

	// IPv6 link-local (fe80::/10)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return AccessLAN
	}

	// IPv6 unique local (fd00::/8)
	_, ula, _ := net.ParseCIDR("fd00::/8")
	if ula.Contains(ip) {
		return AccessLAN
	}

	return AccessRemote
}

func isPrivateIPv4(ip net.IP) bool {
	_, ten, _ := net.ParseCIDR("10.0.0.0/8")
	_, one72, _ := net.ParseCIDR("172.16.0.0/12")
	_, one92, _ := net.ParseCIDR("192.168.0.0/16")
	return ten.Contains(ip) || one72.Contains(ip) || one92.Contains(ip)
}

// getAccessLevelFromRequest determines access level from the HTTP request.
// Special handling for 127.0.0.1: Cloudflare Tunnel traffic also comes from
// 127.0.0.1 (via cloudflared), so we check CF headers to distinguish.
func getAccessLevelFromRequest(r *http.Request) AccessLevel {
	clientIP := getClientIP(r, false)
	level := getAccessLevel(clientIP)

	// If not loopback, return the level directly
	ip := net.ParseIP(clientIP)
	if ip == nil || !ip.IsLoopback() {
		return level
	}

	// Loopback IP: could be local browser or Cloudflare Tunnel.
	// Cloudflare passes CF-Connecting-IP and CF-Ray headers.
	if r.Header.Get("CF-Connecting-IP") != "" || r.Header.Get("CF-Ray") != "" {
		return AccessRemote
	}

	return AccessLocal
}

// accessLevelString returns the string representation for JSON responses.
func accessLevelString(level AccessLevel) string {
	switch level {
	case AccessLocal:
		return "local"
	case AccessLAN:
		return "lan"
	case AccessRemote:
		return "remote"
	default:
		return "remote"
	}
}

// isAdminAPI returns true for management endpoints that should be restricted.
func isAdminAPI(path, method string) bool {
	if path == "/api/config" && method == http.MethodPost {
		return true
	}
	if path == "/api/shares" && method == http.MethodPost {
		return true
	}
	return false
}

// WithAdminLockdown restricts management APIs to local access only.
func WithAdminLockdown(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		level := getAccessLevelFromRequest(r)

		// Local access: unrestricted
		if level == AccessLocal {
			next.ServeHTTP(w, r)
			return
		}

		// LAN or Remote: block admin APIs
		if isAdminAPI(r.URL.Path, r.Method) {
			writeError(w, http.StatusForbidden, constants.ErrMsgAccessDenied)
			return
		}

		next.ServeHTTP(w, r)
	})
}
