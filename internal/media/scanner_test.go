package media

import (
	"testing"
)

func TestClassifyExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".mp4", "video"},
		{".MP4", "other"}, // Function assumes lowercase input
		{".mp3", "audio"},
		{".jpg", "image"},
		{".txt", "other"},
	}

	for _, tt := range tests {
		got := ClassifyExt(tt.ext)
		if got != tt.expected {
			t.Errorf("ClassifyExt(%s) = %s, expected %s", tt.ext, got, tt.expected)
		}
	}
}

func TestIsBlockedSize(t *testing.T) {
	tests := []struct {
		size     int64
		rule     string
		expected bool
	}{
		{100, ">50", true},
		{100, "< 50", false},
		{100, "50-150", true},
		{200, "50-150", false},
	}

	for _, tt := range tests {
		got := IsBlockedSize(tt.size, tt.rule)
		if got != tt.expected {
			t.Errorf("IsBlockedSize(%d, %s) = %v, expected %v", tt.size, tt.rule, got, tt.expected)
		}
	}
}

func TestIsBlockedString(t *testing.T) {
	list := []string{"System Volume Information", "$RECYCLE.BIN", "/^\\./"}
	
	if IsBlockedString(list, "foo") {
		t.Error("foo should not be blocked")
	}
	if !IsBlockedString(list, "System Volume Information") {
		t.Error("System Volume Information should be blocked")
	}
	if !IsBlockedString(list, ".git") {
		t.Error(".git should be blocked by regex")
	}
}
