package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"msp/internal/config"
	"msp/internal/server"
)

func TestIPFiltering(t *testing.T) {
	tests := []struct {
		name        string
		clientIP    string
		whitelist   []string
		blacklist   []string
		shouldAllow bool
	}{
		{
			name:        "Empty lists - allow all",
			clientIP:    "192.168.1.100",
			whitelist:   []string{},
			blacklist:   []string{},
			shouldAllow: true,
		},
		{
			name:        "Whitelist exact match",
			clientIP:    "192.168.1.100",
			whitelist:   []string{"192.168.1.100"},
			blacklist:   []string{},
			shouldAllow: true,
		},
		{
			name:        "Whitelist no match",
			clientIP:    "192.168.1.100",
			whitelist:   []string{"192.168.1.101"},
			blacklist:   []string{},
			shouldAllow: false,
		},
		{
			name:        "Whitelist CIDR /24 match",
			clientIP:    "192.168.1.100",
			whitelist:   []string{"192.168.1.0/24"},
			blacklist:   []string{},
			shouldAllow: true,
		},
		{
			name:        "Whitelist CIDR /24 no match",
			clientIP:    "192.168.2.100",
			whitelist:   []string{"192.168.1.0/24"},
			blacklist:   []string{},
			shouldAllow: false,
		},
		{
			name:        "Blacklist exact match",
			clientIP:    "192.168.1.100",
			whitelist:   []string{},
			blacklist:   []string{"192.168.1.100"},
			shouldAllow: false,
		},
		{
			name:        "Blacklist CIDR match",
			clientIP:    "192.168.1.100",
			whitelist:   []string{},
			blacklist:   []string{"192.168.1.0/24"},
			shouldAllow: false,
		},
		{
			name:        "Whitelist and blacklist - blacklist wins",
			clientIP:    "192.168.1.100",
			whitelist:   []string{"192.168.1.0/24"},
			blacklist:   []string{"192.168.1.100"},
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIPAllowed(tt.clientIP, tt.whitelist, tt.blacklist)
			if result != tt.shouldAllow {
				t.Errorf("isIPAllowed() = %v, want %v", result, tt.shouldAllow)
			}
		})
	}
}

func TestMatchesCIDR(t *testing.T) {
	tests := []struct {
		name     string
		clientIP string
		cidr     string
		want     bool
	}{
		{
			name:     "/24 match",
			clientIP: "192.168.1.100",
			cidr:     "192.168.1.0/24",
			want:     true,
		},
		{
			name:     "/24 no match",
			clientIP: "192.168.2.100",
			cidr:     "192.168.1.0/24",
			want:     false,
		},
		{
			name:     "/16 match",
			clientIP: "192.168.100.50",
			cidr:     "192.168.0.0/16",
			want:     true,
		},
		{
			name:     "/16 no match",
			clientIP: "192.169.1.1",
			cidr:     "192.168.0.0/16",
			want:     false,
		},
		{
			name:     "/8 match",
			clientIP: "10.20.30.40",
			cidr:     "10.0.0.0/8",
			want:     true,
		},
		{
			name:     "/8 no match",
			clientIP: "11.0.0.1",
			cidr:     "10.0.0.0/8",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesCIDR(tt.clientIP, tt.cidr)
			if result != tt.want {
				t.Errorf("matchesCIDR(%s, %s) = %v, want %v", tt.clientIP, tt.cidr, result, tt.want)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100"},
			want:       "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100, 10.0.0.1"},
			want:       "192.168.1.100",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Real-IP": "192.168.1.100"},
			want:       "192.168.1.100",
		},
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.100:5678",
			headers:    map[string]string{},
			want:       "192.168.1.100",
		},
		{
			name:       "IPv6 RemoteAddr",
			remoteAddr: "[::1]:5678",
			headers:    map[string]string{},
			want:       "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := getClientIP(req)
			if result != tt.want {
				t.Errorf("getClientIP() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestRequiresPIN(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "/api/pin exempt",
			path: "/api/pin",
			want: false,
		},
		{
			name: "root path does not require PIN",
			path: "/",
			want: false,
		},
		{
			name: "static resource does not require PIN",
			path: "/assets/index.js",
			want: false,
		},
		{
			name: "favicon does not require PIN",
			path: "/favicon.ico",
			want: false,
		},
		{
			name: "API path requires PIN",
			path: "/api/config",
			want: true,
		},
		{
			name: "API media requires PIN",
			path: "/api/media",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := requiresPIN(tt.path)
			if result != tt.want {
				t.Errorf("requiresPIN(%s) = %v, want %v", tt.path, result, tt.want)
			}
		})
	}
}

func TestWithSecurityMiddleware(t *testing.T) {
	// Create a test server
	s := server.New("test_config.json")

	// Update config with security settings
	_ = s.UpdateConfig(func(cfg *config.Config) {
		cfg.Security.IPWhitelist = []string{"192.168.1.0/24"}
		cfg.Security.PINEnabled = false
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	secureHandler := WithSecurity(s, handler)

	tests := []struct {
		name           string
		remoteAddr     string
		expectedStatus int
	}{
		{
			name:           "Allowed IP",
			remoteAddr:     "192.168.1.100:1234",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Blocked IP",
			remoteAddr:     "10.0.0.1:1234",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			w := httptest.NewRecorder()

			secureHandler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
