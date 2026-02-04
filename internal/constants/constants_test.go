package constants

import (
	"testing"
	"time"
)

func TestDefaultValues(t *testing.T) {
	if DefaultPort != 8099 {
		t.Errorf("DefaultPort = %d, want 8099", DefaultPort)
	}
	if DefaultPIN != "0000" {
		t.Errorf("DefaultPIN = %s, want 0000", DefaultPIN)
	}
}

func TestFilePermissions(t *testing.T) {
	if FilePerm != 0600 {
		t.Errorf("FilePerm = %o, want 0600", FilePerm)
	}
	if DirPerm != 0750 {
		t.Errorf("DirPerm = %o, want 0750", DirPerm)
	}
}

func TestLogConstants(t *testing.T) {
	expectedRotateSize := 10 * 1024 * 1024
	if LogRotateSize != expectedRotateSize {
		t.Errorf("LogRotateSize = %d, want %d", LogRotateSize, expectedRotateSize)
	}
	if LogRotateCheckInterval != 100 {
		t.Errorf("LogRotateCheckInterval = %d, want 100", LogRotateCheckInterval)
	}
	if LogTimeFormat != "2006/01/02 15:04:05.000000" {
		t.Errorf("LogTimeFormat = %s, want specific format", LogTimeFormat)
	}
}

func TestCacheConstants(t *testing.T) {
	if MediaCacheTTL != 2*time.Minute {
		t.Errorf("MediaCacheTTL = %v, want 2m", MediaCacheTTL)
	}
	if ConfigCheckInterval != 2*time.Second {
		t.Errorf("ConfigCheckInterval = %v, want 2s", ConfigCheckInterval)
	}
	if CookieMaxAge != 86400*7 {
		t.Errorf("CookieMaxAge = %d, want %d", CookieMaxAge, 86400*7)
	}
}

func TestScanLimits(t *testing.T) {
	if DefaultScanLimit != 100000 {
		t.Errorf("DefaultScanLimit = %d, want 100000", DefaultScanLimit)
	}
	if DBScanLimit != 1000000000 {
		t.Errorf("DBScanLimit = %d, want 1000000000", DBScanLimit)
	}
}

func TestByteUnits(t *testing.T) {
	if BytesPerKB != 1024 {
		t.Errorf("BytesPerKB = %d, want 1024", BytesPerKB)
	}
	if BytesPerMB != 1024*1024 {
		t.Errorf("BytesPerMB = %d, want %d", BytesPerMB, 1024*1024)
	}
	if BytesPerGB != 1024*1024*1024 {
		t.Errorf("BytesPerGB = %d, want %d", BytesPerGB, 1024*1024*1024)
	}
}

func TestHTTPStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{"StatusOK", StatusOK, 200},
		{"StatusBadRequest", StatusBadRequest, 400},
		{"StatusUnauthorized", StatusUnauthorized, 401},
		{"StatusForbidden", StatusForbidden, 403},
		{"StatusNotFound", StatusNotFound, 404},
		{"StatusInternalServerError", StatusInternalServerError, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.value, tt.expected)
			}
		})
	}
}

func TestNetworkAddresses(t *testing.T) {
	if LocalhostIPv4 != "127.0.0.1" {
		t.Errorf("LocalhostIPv4 = %s, want 127.0.0.1", LocalhostIPv4)
	}
	if LocalhostIPv6 != "::1" {
		t.Errorf("LocalhostIPv6 = %s, want ::1", LocalhostIPv6)
	}
}

func TestPrivateIPRanges(t *testing.T) {
	if PrivateClassA != 10 {
		t.Errorf("PrivateClassA = %d, want 10", PrivateClassA)
	}
	if PrivateClassBStart != 172 {
		t.Errorf("PrivateClassBStart = %d, want 172", PrivateClassBStart)
	}
	if PrivateClassBMin != 16 || PrivateClassBMax != 31 {
		t.Errorf("PrivateClassB range = %d-%d, want 16-31", PrivateClassBMin, PrivateClassBMax)
	}
	if PrivateClassC1 != 192 || PrivateClassC2 != 168 {
		t.Errorf("PrivateClassC = %d.%d, want 192.168", PrivateClassC1, PrivateClassC2)
	}
}
