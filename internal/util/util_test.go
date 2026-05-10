package util

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"msp/internal/domain"
)

func TestEncodeDecodeID(t *testing.T) {
	tests := []string{
		"/path/to/file.mp4",
		"C:\\Windows\\file.mp4",
		"/very/long/path/" + string(make([]byte, 1000)),
		"unicode_文件_测试",
	}

	for _, path := range tests {
		encoded := EncodeID(path)
		decoded, err := DecodeID(encoded)
		if err != nil {
			t.Errorf("DecodeID(%q) error: %v", encoded, err)
			continue
		}
		if decoded != path {
			t.Errorf("DecodeID(EncodeID(%q)) = %q, want %q", path, decoded, path)
		}
	}
}

func TestDecodeID_Invalid(t *testing.T) {
	tests := []string{
		"",
		"!!!",
		"not-valid-base64!!!",
	}

	for _, id := range tests {
		_, err := DecodeID(id)
		if err == nil {
			t.Errorf("DecodeID(%q) expected error, got nil", id)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	// 测试基本路径规范化
	path := NormalizePath("/some/path")
	if path == "" {
		t.Error("NormalizePath returned empty string for valid path")
	}

	// 测试空路径
	empty := NormalizePath("")
	if empty != "" {
		t.Errorf("NormalizePath(\"\") = %q, want empty", empty)
	}

	// 测试带引号的路径
	quoted := NormalizePath(`"/path/with/quotes"`)
	if quoted == `"/path/with/quotes"` {
		t.Error("NormalizePath did not remove quotes")
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"100", 100},
		{"1 KB", 1024},
		{"1.5 MB", 1572864}, // 1.5 * 1024 * 1024
		{"1GB", 1073741824},
		{"2 TB", 2199023255552},
		{"100 B", 100},
		{"", 0},
		{"  1 GB  ", 1073741824},
	}

	for _, tt := range tests {
		got := ParseSize(tt.input)
		if got != tt.expected {
			t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestU64Base36(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{35, "z"},
		{36, "10"},
		{100, "2s"},
		{123456789, "21i3v9"},
	}

	for _, tt := range tests {
		got := U64Base36(tt.input)
		if got != tt.expected {
			t.Errorf("U64Base36(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{12345, "12345"},
		{-12345, "-12345"},
	}

	for _, tt := range tests {
		got := Itoa(tt.input)
		if got != tt.expected {
			t.Errorf("Itoa(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDedupeShares(t *testing.T) {
	shares := []domain.Share{
		{Label: "A", Path: "/path/a"},
		{Label: "B", Path: "/path/b"},
		{Label: "A2", Path: "/PATH/A"}, // duplicate case-insensitive
	}

	result := DedupeShares(shares)
	if len(result) != 2 {
		t.Errorf("DedupeShares returned %d items, want 2", len(result))
	}
}

func TestNormalizeShares(t *testing.T) {
	shares := []domain.Share{
		{Label: "", Path: "/valid/path"},
		{Label: "Custom", Path: "/another/path"},
		{Label: "Empty", Path: ""},
	}

	result := NormalizeShares(shares)
	// Should filter out empty paths and generate labels
	if len(result) == 0 {
		t.Error("NormalizeShares returned empty result")
	}
}

func TestIsExistingDir(t *testing.T) {
	// Test with current directory (should exist)
	if !IsExistingDir(".") {
		t.Error("IsExistingDir(\".\") = false, want true")
	}

	// Test with non-existent directory
	if IsExistingDir("/nonexistent/path/that/does/not/exist") {
		t.Error("IsExistingDir(nonexistent) = true, want false")
	}
}

func TestWithinRoot(t *testing.T) {
	tests := []struct {
		root     string
		target   string
		expected bool
	}{
		{"/home", "/home/user/file.txt", true},
		{"/home", "/home", true},
		{"/home", "/etc/passwd", false},
		{"/home/user", "/home/user2", false},
	}

	for _, tt := range tests {
		got := WithinRoot(tt.root, tt.target)
		if got != tt.expected {
			t.Errorf("WithinRoot(%q, %q) = %v, want %v", tt.root, tt.target, got, tt.expected)
		}
	}
}

func TestSamePath(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"/home/user", "/home/user", true},
		{"/home/user/", "/home/user", true},
		{"/home/user", "/home/other", false},
	}

	for _, tt := range tests {
		got := SamePath(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("SamePath(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestIsPrivateIPv4(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},
		{"192.169.1.1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Errorf("Failed to parse IP: %s", tt.ip)
			continue
		}
		ip4 := ip.To4()
		if ip4 == nil {
			t.Errorf("Failed to convert to IPv4: %s", tt.ip)
			continue
		}
		got := IsPrivateIPv4(ip4)
		if got != tt.expected {
			t.Errorf("IsPrivateIPv4(%s) = %v, want %v", tt.ip, got, tt.expected)
		}
	}
}

func TestDedupeStrings(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "d"}
	expected := []string{"a", "b", "c", "d"}

	result := DedupeStrings(input)
	if len(result) != len(expected) {
		t.Errorf("DedupeStrings length = %d, want %d", len(result), len(expected))
	}

	seen := make(map[string]bool)
	for _, s := range result {
		if seen[s] {
			t.Errorf("DedupeStrings returned duplicate: %s", s)
		}
		seen[s] = true
	}
}

func TestIsAllowedFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 在 share 目录内创建测试文件
	testFile := filepath.Join(tmpDir, "video.mp4")
	if err := os.WriteFile(testFile, []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	subFile := filepath.Join(subDir, "audio.mp3")
	if err := os.WriteFile(subFile, []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	shares := []domain.Share{{Label: "Media", Path: tmpDir}}

	t.Run("empty path returns false", func(t *testing.T) {
		if IsAllowedFile("", shares) {
			t.Error("empty path should not be allowed")
		}
	})

	t.Run("file in share root is allowed", func(t *testing.T) {
		if !IsAllowedFile(testFile, shares) {
			t.Errorf("file %q should be allowed", testFile)
		}
	})

	t.Run("file in subdirectory of share is allowed", func(t *testing.T) {
		if !IsAllowedFile(subFile, shares) {
			t.Errorf("subdirectory file %q should be allowed", subFile)
		}
	})

	t.Run("directory path returns false", func(t *testing.T) {
		if IsAllowedFile(tmpDir, shares) {
			t.Error("directory path should not be allowed")
		}
	})

	t.Run("file outside shares returns false", func(t *testing.T) {
		otherDir := t.TempDir()
		otherFile := filepath.Join(otherDir, "other.mp4")
		if err := os.WriteFile(otherFile, []byte("data"), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if IsAllowedFile(otherFile, shares) {
			t.Error("file outside shares should not be allowed")
		}
	})

	t.Run("non-existent file returns false", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "doesnotexist.mp4")
		if IsAllowedFile(nonExistent, shares) {
			t.Error("non-existent file should not be allowed")
		}
	})

	t.Run("empty shares returns false", func(t *testing.T) {
		if IsAllowedFile(testFile, []domain.Share{}) {
			t.Error("file should not be allowed when shares is empty")
		}
	})
}
