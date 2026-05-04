package service

import (
	"context"
	"testing"
	"time"

	"msp/internal/cache"
	"msp/internal/config"
	"msp/internal/domain"
)

type mockMediaConfigReader struct {
	cfg config.Config
}

func (m *mockMediaConfigReader) Config() config.Config {
	return m.cfg
}

func TestNewMediaService(t *testing.T) {
	c := cache.NewMediaCache("/tmp/test.json", 5*time.Minute)
	cfg := &mockMediaConfigReader{cfg: config.Default()}
	svc := NewMediaService(c, cfg)
	if svc == nil {
		t.Fatal("NewMediaService returned nil")
	}
}

func TestMediaServiceInvalidateMediaCache(t *testing.T) {
	c := cache.NewMediaCache("/tmp/test.json", 5*time.Minute)
	cfg := &mockMediaConfigReader{cfg: config.Default()}
	svc := NewMediaService(c, cfg)

	svc.InvalidateMediaCache()
}

func TestMediaServiceGetOrBuildMediaCache(t *testing.T) {
	c := cache.NewMediaCache(t.TempDir()+"/cache.json", 5*time.Minute)
	cfg := &mockMediaConfigReader{cfg: config.Default()}
	svc := NewMediaService(c, cfg)

	ctx := context.Background()
	shares := []domain.Share{{Label: "Test", Path: t.TempDir()}}
	resp, etag := svc.GetOrBuildMediaCache(ctx, shares, config.BlacklistConfig{}, false)
	if etag == "" {
		t.Error("etag should not be empty after build")
	}
	_ = resp
}

func TestMediaServiceLoadMediaCacheFromDisk(t *testing.T) {
	c := cache.NewMediaCache(t.TempDir()+"/cache.json", 5*time.Minute)
	cfg := &mockMediaConfigReader{cfg: config.Default()}
	svc := NewMediaService(c, cfg)

	result := svc.LoadMediaCacheFromDisk()
	if result {
		t.Error("expected false when no cache file exists")
	}
}
