package util

import (
	"strings"
	"testing"
)

func TestEncodeDecodeID(t *testing.T) {
	path := "/usr/local/bin/test"
	encoded := EncodeID(path)
	decoded, err := DecodeID(encoded)
	if err != nil {
		t.Fatalf("DecodeID failed: %v", err)
	}
	if decoded != path {
		t.Errorf("Expected %s, got %s", path, decoded)
	}
}

func TestNormalizePath(t *testing.T) {
	// Only test basic cleaning, as absolute path depends on OS/CWD
	input := "foo//bar/../baz"
	normalized := NormalizePath(input)
	if !strings.Contains(normalized, "baz") {
		t.Errorf("Normalization failed for %s: got %s", input, normalized)
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
	}

	for _, tt := range tests {
		val := ParseSize(tt.input)
		if val != tt.expected {
			t.Errorf("ParseSize(%s) = %d, expected %d", tt.input, val, tt.expected)
		}
	}
}
