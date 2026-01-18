package service

import (
	"msp/internal/config"
	"msp/internal/media"
	"msp/internal/server"
	"msp/internal/util"
	"path/filepath"
	"time"
)

type ConfigService struct {
	s *server.Server
}

func NewConfigService(s *server.Server) *ConfigService {
	return &ConfigService{s: s}
}

// ConfigView 包含了前端所需的配置和环境信息
type ConfigView struct {
	Config           config.Config `json:"config"`
	LanIPs           []string      `json:"lanIPs"`
	URLs             []string      `json:"urls"`
	NowUnix          int64         `json:"nowUnix"`
	FFmpegAvailable  bool          `json:"ffmpegAvailable"`
	FFprobeAvailable bool          `json:"ffprobeAvailable"`
}

func (s *ConfigService) GetConfigView() ConfigView {
	ips := util.GetLanIPv4s()
	port := s.s.GetPort()
	urls := make([]string, 0, 2+len(ips))
	urls = append(urls, "http://127.0.0.1:"+util.Itoa(port)+"/")
	for _, ip := range ips {
		urls = append(urls, "http://"+ip+":"+util.Itoa(port)+"/")
	}

	return ConfigView{
		Config:           s.s.Config(),
		LanIPs:           ips,
		URLs:             urls,
		NowUnix:          time.Now().Unix(),
		FFmpegAvailable:  media.CheckFFmpeg(),
		FFprobeAvailable: media.CheckFFprobe(),
	}
}

func (s *ConfigService) UpdateConfig(cfg config.Config) (config.Config, error) {
	config.ApplyDefaults(&cfg)
	cfg.Shares = util.NormalizeShares(cfg.Shares)

	validShares := make([]config.Share, 0, len(cfg.Shares))
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

	err := s.s.UpdateConfig(func(c *config.Config) {
		*c = cfg
	})
	if err != nil {
		return config.Config{}, err
	}

	s.s.InvalidateMediaCache()
	return cfg, nil
}
