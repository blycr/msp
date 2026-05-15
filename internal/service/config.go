package service

import (
	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/media"
	"msp/internal/util"
	"path/filepath"
	"time"
)

type ConfigProvider interface {
	Config() config.Config
	GetPort() int
	UpdateConfig(func(*config.Config)) error
}

type MediaCacheInvalidator interface {
	InvalidateMediaCache()
}

type ConfigService struct {
	config    ConfigProvider
	cache     MediaCacheInvalidator
	processor *media.MediaProcessor
}

func NewConfigService(config ConfigProvider, cache MediaCacheInvalidator, processor *media.MediaProcessor) *ConfigService {
	return &ConfigService{config: config, cache: cache, processor: processor}
}

// SecurityConfigView is the safe view of security configuration (hides sensitive info)
type SecurityConfigView struct {
	IPWhitelist []string `json:"ipWhitelist"`
	IPBlacklist []string `json:"ipBlacklist"`
	PINEnabled  bool     `json:"pinEnabled"`
}

// ConfigView contains frontend-facing config and environment info (safe version)
type ConfigView struct {
	Config           SafeConfig `json:"config"`
	LanIPs           []string   `json:"lanIPs"`
	URLs             []string   `json:"urls"`
	NowUnix          int64      `json:"nowUnix"`
	FFmpegAvailable  bool       `json:"ffmpegAvailable"`
	FFprobeAvailable bool       `json:"ffprobeAvailable"`
	AccessLevel      string     `json:"accessLevel"`
}

// SafeConfig is the safe view of Config, hiding sensitive information
type SafeConfig struct {
	Port      int                    `json:"port"`
	LogLevel  string                 `json:"logLevel"`
	LogFile   string                 `json:"logFile"`
	MaxItems  int                    `json:"maxItems"`
	Shares    []domain.Share         `json:"shares"`
	Features  config.Features        `json:"features"`
	UI        config.UIConfig        `json:"ui"`
	Playback  config.PlaybackConfig  `json:"playback"`
	Security  SecurityConfigView     `json:"security"`
	Blacklist config.BlacklistConfig `json:"blacklist"`
}

// toSafeConfig converts Config to its safe view
func toSafeConfig(cfg config.Config) SafeConfig {
	return SafeConfig{
		Port:     cfg.Port,
		LogLevel: cfg.LogLevel,
		LogFile:  cfg.LogFile,
		MaxItems: cfg.MaxItems,
		Shares:   cfg.Shares,
		Features: cfg.Features,
		UI:       cfg.UI,
		Playback: cfg.Playback,
		Security: SecurityConfigView{
			IPWhitelist: cfg.Security.IPWhitelist,
			IPBlacklist: cfg.Security.IPBlacklist,
			PINEnabled:  cfg.Security.PINEnabled,
		},
		Blacklist: cfg.Blacklist,
	}
}

func (s *ConfigService) GetConfigView() ConfigView {
	ips := util.GetLanIPv4s()
	port := s.config.GetPort()
	urls := make([]string, 0, 2+len(ips))
	urls = append(urls, "http://127.0.0.1:"+util.Itoa(port)+"/")
	for _, ip := range ips {
		urls = append(urls, "http://"+ip+":"+util.Itoa(port)+"/")
	}

	return ConfigView{
		Config:           toSafeConfig(s.config.Config()),
		LanIPs:           ips,
		URLs:             urls,
		NowUnix:          time.Now().Unix(),
		FFmpegAvailable:  s.processor.CheckFFmpeg(),
		FFprobeAvailable: s.processor.CheckFFprobe(),
	}
}

func (s *ConfigService) UpdateConfig(cfg config.Config) (config.Config, error) {
	config.ApplyDefaults(&cfg)
	cfg.Shares = util.NormalizeShares(cfg.Shares)

	validShares := make([]domain.Share, 0, len(cfg.Shares))
	for _, sh := range cfg.Shares {
		if sh.Path == "" {
			continue
		}
		p := util.NormalizePath(sh.Path)
		if ok := util.IsExistingDir(p); !ok {
			continue
		}
		sh.Path = p
		if sh.Label == "" {
			sh.Label = filepath.Base(p)
		}
		validShares = append(validShares, sh)
	}
	cfg.Shares = util.DedupeShares(validShares)

	err := s.config.UpdateConfig(func(c *config.Config) {
		*c = cfg
	})
	if err != nil {
		return config.Config{}, err
	}

	s.cache.InvalidateMediaCache()
	return cfg, nil
}
