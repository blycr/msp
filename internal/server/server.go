package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"msp/internal/cache"
	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/media"
	"msp/internal/service"
)

type Server struct {
	mu         sync.RWMutex
	cfg        config.Config
	cfgPath    string
	cfgModTime time.Time

	MediaSvc  *service.MediaService
	session   *service.SessionService
	logger    *service.LoggerService
	processor *media.MediaProcessor

	// refreshMedia triggers an async media cache rebuild after a Shares
	// change. Replaceable in tests.
	refreshMedia func(config.Config)
}

func New(cfgPath string, processor *media.MediaProcessor) *Server {
	s := &Server{
		cfgPath:   cfgPath,
		session:   service.NewSessionService(),
		processor: processor,
	}
	s.logger = service.NewLoggerService("", "")
	s.MediaSvc = service.NewMediaService(
		cache.NewMediaCache(processor, cache.FormatMediaCachePath(cfgPath), config.DefaultMediaCacheTTL),
		s,
	)
	s.refreshMedia = s.refreshMediaCacheAsync
	return s
}

func (s *Server) LoadOrInitConfig() error {
	b, err := os.ReadFile(s.cfgPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		cfg := config.Default()
		s.mu.Lock()
		s.cfg = cfg
		s.mu.Unlock()
		return s.saveConfigLocked()
	}

	stat, err := os.Stat(s.cfgPath)
	if err == nil {
		s.mu.Lock()
		s.cfgModTime = stat.ModTime()
		s.mu.Unlock()
	}

	var cfg config.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}

	changed := config.ApplyDefaults(&cfg)
	oldPIN := cfg.Security.PIN
	config.SanitizeSecurity(&cfg)
	if oldPIN != "" && cfg.Security.PIN == "" {
		changed = true
	}
	if errs := config.Validate(&cfg); len(errs) > 0 {
		slog.Warn("loaded config failed validation; continuing", "err", errs[0])
	}

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	if changed {
		s.Log("info", "Config updated with default values and saved to disk")
		return s.saveConfigLocked()
	}
	return nil
}

func (s *Server) saveConfigLocked() error {
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.cfgPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.cfgPath); err != nil {
		return err
	}
	if st, err := os.Stat(s.cfgPath); err == nil {
		s.cfgModTime = st.ModTime()
	}
	return nil
}

func (s *Server) Config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Clone()
}

func (s *Server) UpdateConfig(fn func(*config.Config)) error {
	s.mu.Lock()
	old := s.cfg
	fn(&s.cfg)
	config.ApplyDefaults(&s.cfg)
	config.SanitizeSecurity(&s.cfg)
	err := s.saveConfigLocked()
	newCfg := s.cfg
	s.mu.Unlock()

	s.applyConfigDiff(old, newCfg)
	return err
}

// applyConfigDiff applies side effects for a replaced configuration. It must
// be called outside s.mu, after s.cfg has been fully validated and swapped.
// Side-effect failures never roll back the config replacement.
func (s *Server) applyConfigDiff(old, newCfg config.Config) {
	if old.LogLevel != newCfg.LogLevel || old.LogFile != newCfg.LogFile {
		s.logger.UpdateConfig(newCfg.LogLevel, newCfg.LogFile)
		if old.LogFile != newCfg.LogFile {
			if err := s.logger.Reopen(); err != nil {
				slog.Warn("failed to reopen log file", "err", err)
			}
		}
	}

	if !sharesEqual(old.Shares, newCfg.Shares) && s.refreshMedia != nil {
		s.refreshMedia(newCfg)
	}

	if old.Port != newCfg.Port {
		slog.Warn("port change requires restart", "old", old.Port, "new", newCfg.Port)
	}

	if !reflect.DeepEqual(old.Playback.Video.Encoding, newCfg.Playback.Video.Encoding) {
		slog.Warn("encoding config change requires restart",
			"old", old.Playback.Video.Encoding, "new", newCfg.Playback.Video.Encoding)
	}
}

// sharesEqual compares two share lists, treating nil and empty as equal so
// that ApplyDefaults normalization does not look like a change.
func sharesEqual(a, b []domain.Share) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// refreshMediaCacheAsync rebuilds the media cache in the background so a
// config reload is never blocked by scanning. The rebuild itself is spawned
// and tracked inside the cache; this call returns immediately.
func (s *Server) refreshMediaCacheAsync(cfg config.Config) {
	s.MediaSvc.RefreshMediaCache(cfg.Shares, cfg.Blacklist)
}

func (s *Server) WatchConfig(ctx context.Context) {
	ticker := time.NewTicker(config.DefaultConfigCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 服务关闭：停止扫描派生的后台预热。
			s.processor.CancelPostScanHook()
			return
		case <-ticker.C:
			s.checkAndReloadConfig()
		}
	}
}

func (s *Server) checkAndReloadConfig() {
	stat, err := os.Stat(s.cfgPath)
	if err != nil {
		return
	}

	s.mu.RLock()
	lastModTime := s.cfgModTime
	s.mu.RUnlock()

	if !stat.ModTime().After(lastModTime) {
		return
	}

	b, err := os.ReadFile(s.cfgPath)
	if err != nil {
		s.Log("error", "Failed to read config file: "+err.Error())
		return
	}

	var cfg config.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		s.Log("error", "Failed to parse config file: "+err.Error())
		return
	}

	config.ApplyDefaults(&cfg)

	oldPIN := cfg.Security.PIN
	config.SanitizeSecurity(&cfg)
	if errs := config.Validate(&cfg); len(errs) > 0 {
		s.Log("error", "Ignoring invalid config reload: "+errs[0].Error())
		return
	}
	needsSave := oldPIN != "" && cfg.Security.PIN == ""

	s.mu.Lock()
	old := s.cfg
	s.cfg = cfg

	// If a plaintext PIN was hashed during reload, persist it back to disk
	// so the next HandlePIN sees the hash instead of an empty PINHash.
	if needsSave {
		if err := s.saveConfigLocked(); err != nil {
			s.Log("error", "Failed to save config after PIN sanitization: "+err.Error())
		}
	} else {
		s.cfgModTime = stat.ModTime()
	}

	s.mu.Unlock()

	s.applyConfigDiff(old, cfg)
	s.Log("info", "Config reloaded successfully")
}

func (s *Server) SetupLogger() {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	s.logger.UpdateConfig(cfg.LogLevel, cfg.LogFile)
	s.logger.SetupLogger()
}

func (s *Server) Log(level string, msg string) {
	s.logger.Log(level, msg)
}

func (s *Server) LogRequest(r *http.Request, status int, start time.Time) {
	s.logger.LogRequest(r, status, start)
}

func (s *Server) GetPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Port <= 0 {
		return config.DefaultPort
	}
	return s.cfg.Port
}

func (s *Server) PeekMediaETag() (string, bool) {
	return s.MediaSvc.PeekMediaETag()
}

func (s *Server) GetOrBuildMediaCache(ctx context.Context, shares []domain.Share, blacklist config.BlacklistConfig, refresh bool) (domain.MediaResponse, string) {
	return s.MediaSvc.GetOrBuildMediaCache(ctx, shares, blacklist, refresh)
}

func (s *Server) InvalidateMediaCache() {
	s.MediaSvc.InvalidateMediaCache()
}

func (s *Server) WaitForBackgroundMediaOps() {
	s.MediaSvc.WaitForBackground()
}

// SetBackgroundContext injects the process-wide background context into the
// media cache so background rebuilds can be cancelled during shutdown.
// Must be called before the server starts handling requests.
func (s *Server) SetBackgroundContext(ctx context.Context) {
	s.MediaSvc.SetBackgroundContext(ctx)
}

func (s *Server) CreateSession() (string, error) {
	return s.session.CreateSession()
}

func (s *Server) ValidateSession(token string) bool {
	return s.session.ValidateSession(token)
}

func (s *Server) Logger() *service.LoggerService {
	return s.logger
}
