package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"msp/internal/config"
	"msp/internal/domain"
)

func TestNewMediaCache(t *testing.T) {
	c := NewMediaCache("/tmp/test.json", 5*time.Minute)
	if c == nil {
		t.Fatal("NewMediaCache returned nil")
	}
	if c.cacheFilePath != "/tmp/test.json" {
		t.Errorf("expected cacheFilePath /tmp/test.json, got %s", c.cacheFilePath)
	}
	if c.ttl != 5*time.Minute {
		t.Errorf("expected ttl 5m, got %v", c.ttl)
	}
}

func TestCacheKey(t *testing.T) {
	t.Run("same shares and blacklist produce same key", func(t *testing.T) {
		shares := []domain.Share{{Label: "Videos", Path: "/media/videos"}}
		bl := config.BlacklistConfig{Extensions: []string{".txt"}, SizeRule: ">100MB"}
		k1 := CacheKey(shares, bl)
		k2 := CacheKey(shares, bl)
		if k1 != k2 {
			t.Error("same inputs should produce same key")
		}
		if k1 == "" {
			t.Error("key should not be empty")
		}
	})

	t.Run("different shares produce different keys", func(t *testing.T) {
		bl := config.BlacklistConfig{}
		k1 := CacheKey([]domain.Share{{Label: "A", Path: "/a"}}, bl)
		k2 := CacheKey([]domain.Share{{Label: "B", Path: "/b"}}, bl)
		if k1 == k2 {
			t.Error("different shares should produce different keys")
		}
	})

	t.Run("different blacklists produce different keys", func(t *testing.T) {
		shares := []domain.Share{{Label: "V", Path: "/v"}}
		k1 := CacheKey(shares, config.BlacklistConfig{Extensions: []string{".txt"}})
		k2 := CacheKey(shares, config.BlacklistConfig{Extensions: []string{".mp3"}})
		if k1 == k2 {
			t.Error("different blacklists should produce different keys")
		}
	})

	t.Run("order of blacklist extensions does not matter", func(t *testing.T) {
		shares := []domain.Share{{Label: "V", Path: "/v"}}
		k1 := CacheKey(shares, config.BlacklistConfig{Extensions: []string{".a", ".b"}})
		k2 := CacheKey(shares, config.BlacklistConfig{Extensions: []string{".b", ".a"}})
		if k1 != k2 {
			t.Error("extension order should not affect key")
		}
	})

	t.Run("empty shares produce valid key", func(t *testing.T) {
		k := CacheKey(nil, config.BlacklistConfig{})
		if k == "" {
			t.Error("key should not be empty even with nil shares")
		}
	})
}

func TestWeakETag(t *testing.T) {
	t.Run("deterministic output", func(t *testing.T) {
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		e1 := WeakETag("key1", ts)
		e2 := WeakETag("key1", ts)
		if e1 != e2 {
			t.Error("same input should produce same ETag")
		}
	})

	t.Run("different keys produce different ETags", func(t *testing.T) {
		ts := time.Now()
		e1 := WeakETag("key1", ts)
		e2 := WeakETag("key2", ts)
		if e1 == e2 {
			t.Error("different keys should produce different ETags")
		}
	})

	t.Run("different times produce different ETags", func(t *testing.T) {
		e1 := WeakETag("key", time.Unix(1000, 0))
		e2 := WeakETag("key", time.Unix(2000, 0))
		if e1 == e2 {
			t.Error("different times should produce different ETags")
		}
	})

	t.Run("starts with W/", func(t *testing.T) {
		e := WeakETag("test", time.Now())
		if len(e) < 2 || e[:2] != "W/" {
			t.Errorf("ETag should start with W/, got %s", e)
		}
	})
}

func TestFormatMediaCachePath(t *testing.T) {
	result := FormatMediaCachePath("/tmp/config.json")
	expected := "/tmp/config.json.media_cache.json"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestInvalidate(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.json")
	_ = os.WriteFile(cachePath, []byte(`{}`), 0644)

	c := NewMediaCache(cachePath, 5*time.Minute)
	c.key = "some-key"
	c.etag = "some-etag"
	c.builtAt = time.Now()
	c.respJSON = []byte(`{"videos":[]}`)

	c.Invalidate()

	if c.key != "" {
		t.Error("key should be empty after invalidate")
	}
	if c.etag != "" {
		t.Error("etag should be empty after invalidate")
	}
	if !c.builtAt.IsZero() {
		t.Error("builtAt should be zero after invalidate")
	}
	if c.respJSON != nil {
		t.Error("respJSON should be nil after invalidate")
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should be removed after invalidate")
	}
}

func TestNormRuleList(t *testing.T) {
	t.Run("trims and lowercases", func(t *testing.T) {
		result := normRuleList([]string{" .TXT ", ".MP3"})
		if len(result) != 2 {
			t.Fatalf("expected 2 items, got %d", len(result))
		}
		if result[0] != ".mp3" || result[1] != ".txt" {
			t.Errorf("expected sorted lowercase [.mp3 .txt], got %v", result)
		}
	})

	t.Run("filters empty strings", func(t *testing.T) {
		result := normRuleList([]string{"", " .mp4 ", ""})
		if len(result) != 1 {
			t.Fatalf("expected 1 item, got %d", len(result))
		}
		if result[0] != ".mp4" {
			t.Errorf("expected .mp4, got %s", result[0])
		}
	})

	t.Run("nil input returns empty slice", func(t *testing.T) {
		result := normRuleList(nil)
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %v", result)
		}
	})
}

func TestSharesCacheKey(t *testing.T) {
	t.Run("shares are sorted by path", func(t *testing.T) {
		shares := []domain.Share{
			{Label: "B", Path: filepath.Join(t.TempDir(), "b")},
			{Label: "A", Path: filepath.Join(t.TempDir(), "a")},
		}
		key := sharesCacheKey(shares)
		lines := splitLines(key)
		if len(lines) < 2 {
			t.Fatalf("expected 2 lines, got %d", len(lines))
		}
		if !contains(lines[0], "a") || !contains(lines[1], "b") {
			t.Errorf("expected sorted order, got lines: %v", lines)
		}
	})

	t.Run("includes label", func(t *testing.T) {
		shares := []domain.Share{{Label: "MyLabel", Path: t.TempDir()}}
		key := sharesCacheKey(shares)
		if !contains(key, "MyLabel") {
			t.Errorf("key should contain label, got %s", key)
		}
	})
}

func TestLoadFromDisk(t *testing.T) {
	t.Run("returns false when file does not exist", func(t *testing.T) {
		c := NewMediaCache(filepath.Join(t.TempDir(), "nonexistent.json"), 5*time.Minute)
		if c.LoadFromDisk("some-key") {
			t.Error("expected false for non-existent file")
		}
	})

	t.Run("returns false for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "cache.json")
		_ = os.WriteFile(p, []byte("not-json"), 0644)

		c := NewMediaCache(p, 5*time.Minute)
		if c.LoadFromDisk("some-key") {
			t.Error("expected false for invalid JSON")
		}
	})

	t.Run("returns false for mismatched key", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "cache.json")
		v := mediaCacheOnDisk{Key: "other-key", BuiltAt: time.Now().UnixNano(), ETag: "etag", Resp: domain.MediaResponse{}}
		b, _ := jsonMarshal(v)
		_ = os.WriteFile(p, b, 0644)

		c := NewMediaCache(p, 5*time.Minute)
		if c.LoadFromDisk("my-key") {
			t.Error("expected false for mismatched key")
		}
	})

	t.Run("loads valid cache file", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "cache.json")
		v := mediaCacheOnDisk{Key: "my-key", BuiltAt: time.Now().UnixNano(), ETag: "etag123", Resp: domain.MediaResponse{}}
		b, _ := jsonMarshal(v)
		_ = os.WriteFile(p, b, 0644)

		c := NewMediaCache(p, 5*time.Minute)
		if !c.LoadFromDisk("my-key") {
			t.Error("expected true for valid cache file")
		}
		if c.key != "my-key" {
			t.Errorf("expected key my-key, got %s", c.key)
		}
		if c.etag != "etag123" {
			t.Errorf("expected etag etag123, got %s", c.etag)
		}
	})
}

func splitLines(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
