package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
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
	return s.cfg
}

func (s *Server) UpdateConfig(fn func(*config.Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.cfg)
	config.ApplyDefaults(&s.cfg)
	config.SanitizeSecurity(&s.cfg)
	return s.saveConfigLocked()
}

func (s *Server) WatchConfig(ctx context.Context) {
	ticker := time.NewTicker(config.DefaultConfigCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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
	needsSave := oldPIN != "" && cfg.Security.PIN == ""

	s.mu.Lock()
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

	s.logger.UpdateConfig(cfg.LogLevel, cfg.LogFile)
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

func (s *Server) CreateSession() (string, error) {
	return s.session.CreateSession()
}

func (s *Server) ValidateSession(token string) bool {
	return s.session.ValidateSession(token)
}

func (s *Server) Logger() *service.LoggerService {
	return s.logger
}
