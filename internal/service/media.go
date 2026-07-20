package service

import (
	"context"

	"msp/internal/cache"
	"msp/internal/config"
	"msp/internal/domain"
)

type ConfigReader interface {
	Config() config.Config
}

type MediaService struct {
	cache  *cache.MediaCache
	config ConfigReader
}

func NewMediaService(cache *cache.MediaCache, config ConfigReader) *MediaService {
	return &MediaService{cache: cache, config: config}
}

func (s *MediaService) PeekMediaETag() (string, bool) {
	return s.cache.PeekETag()
}

func (s *MediaService) GetOrBuildMediaCache(ctx context.Context, shares []domain.Share, blacklist config.BlacklistConfig, refresh bool) (domain.MediaResponse, string) {
	maxItems := s.config.Config().MaxItems
	return s.cache.GetOrBuild(ctx, shares, blacklist, refresh, maxItems)
}

func (s *MediaService) InvalidateMediaCache() {
	s.cache.Invalidate()
}

// RefreshMediaCache asynchronously rebuilds the cache for the given
// shares/blacklist without blocking the caller (e.g. after a config
// hot-reload). The rebuild is tracked by WaitForBackground.
func (s *MediaService) RefreshMediaCache(shares []domain.Share, blacklist config.BlacklistConfig) {
	maxItems := s.config.Config().MaxItems
	s.cache.Refresh(shares, blacklist, maxItems)
}

func (s *MediaService) LoadMediaCacheFromDisk() bool {
	cfg := s.config.Config()
	return s.cache.LoadFromDisk(cache.CacheKey(append([]domain.Share(nil), cfg.Shares...), cfg.Blacklist))
}

func (s *MediaService) WaitForBackground() {
	s.cache.WaitForBackground()
}

// SetBackgroundContext sets the context governing background cache rebuilds
// and disk writes. Must be called before serving traffic.
func (s *MediaService) SetBackgroundContext(ctx context.Context) {
	s.cache.SetBackgroundContext(ctx)
}
