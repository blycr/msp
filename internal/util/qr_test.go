package util

import (
	"strings"
	"testing"
)

func TestQRCodeTerminal(t *testing.T) {
	out, err := QRCodeTerminal("http://192.168.1.100:8099/")
	if err != nil {
		t.Fatalf("QRCodeTerminal error: %v", err)
	}
	if !strings.Contains(out, qrDarkModule) {
		t.Error("expected dark module ANSI block")
	}
	if !strings.Contains(out, qrLightModule) {
		t.Error("expected light module ANSI block")
	}
	if !strings.Contains(out, "\x1b[0m") {
		t.Error("expected ANSI reset sequences")
	}

	// 矩形：每行字符数一致（quiet zone 内四边同为浅色）
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 21 {
		t.Fatalf("QR should be at least 21 module rows, got %d", len(lines))
	}
	width := len(lines[0])
	for i, l := range lines {
		if len(l) != width {
			t.Errorf("line %d width mismatch: got %d, want %d", i, len(l), width)
		}
	}
}

func TestQRCodeTerminalEmpty(t *testing.T) {
	// 空内容应报错而非 panic（正常调用不会传空）
	_, err := QRCodeTerminal("")
	if err == nil {
		t.Error("expected error for empty content")
	}
}
