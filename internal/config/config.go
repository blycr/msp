package config

type Features struct {
	Speed        bool      `json:"speed"`
	SpeedOptions []float64 `json:"speedOptions"`
	Quality      bool      `json:"quality"`
	Captions     bool      `json:"captions"`
	Playlist     bool      `json:"playlist"`
}

type Share struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type UIConfig struct {
	DefaultTab *string `json:"defaultTab"`
	ShowOthers *bool   `json:"showOthers"`
}

type PlaybackAudioConfig struct {
	Enabled   *bool   `json:"enabled"`
	Shuffle   *bool   `json:"shuffle"`
	Remember  *bool   `json:"remember"`
	Scope     *string `json:"scope"`
	Transcode *bool   `json:"transcode"`
}

type PlaybackVideoConfig struct {
	Enabled   *bool   `json:"enabled"`
	Scope     *string `json:"scope"`
	Transcode *bool   `json:"transcode"`
	Resume    *bool   `json:"resume"`
}

type PlaybackImageConfig struct {
	Enabled *bool   `json:"enabled"`
	Scope   *string `json:"scope"`
}

type PlaybackConfig struct {
	Audio PlaybackAudioConfig `json:"audio"`
	Video PlaybackVideoConfig `json:"video"`
	Image PlaybackImageConfig `json:"image"`
}

type BlacklistConfig struct {
	Extensions []string `json:"extensions"`
	Filenames  []string `json:"filenames"`
	Folders    []string `json:"folders"`
	SizeRule   string   `json:"sizeRule"`
}

// SecurityConfig defines IP filtering and PIN authentication settings
type SecurityConfig struct {
	// IPWhitelist is a list of allowed IP addresses or CIDR ranges
	// If not empty, only IPs in this list can access the server
	IPWhitelist []string `json:"ipWhitelist"`

	// IPBlacklist is a list of blocked IP addresses or CIDR ranges
	// IPs in this list will be denied access
	IPBlacklist []string `json:"ipBlacklist"`

	// PINEnabled enables PIN authentication
	PINEnabled bool `json:"pinEnabled"`

	// PIN is the authentication code (default: "0000")
	PIN string `json:"pin"`
}

type Config struct {
	Port      int             `json:"port"`
	Shares    []Share         `json:"shares"`
	Features  Features        `json:"features"`
	UI        UIConfig        `json:"ui"`
	Playback  PlaybackConfig  `json:"playback"`
	Blacklist BlacklistConfig `json:"blacklist"`
	Security  SecurityConfig  `json:"security"`
	LogLevel  string          `json:"logLevel"`
	LogFile   string          `json:"logFile"`
	MaxItems  int             `json:"maxItems"`
}

func boolPtr(v bool) *bool { return &v }

func stringPtr(v string) *string { return &v }

// Default configuration values
func Default() Config {
	return Config{
		Port:     8099,
		MaxItems: 0, // 0 means unlimited (full scan), ideal for SQLite-backed incremental scanning
		Shares:   []Share{},
		Features: Features{
			Speed:        true,
			SpeedOptions: []float64{0.5, 0.75, 1, 1.25, 1.5, 2},
			Quality:      false,
			Captions:     true,
			Playlist:     true,
		},
		UI: UIConfig{
			DefaultTab: stringPtr("video"),
			ShowOthers: boolPtr(false),
		},
		Playback: PlaybackConfig{
			Audio: PlaybackAudioConfig{
				Enabled:   boolPtr(true),
				Shuffle:   boolPtr(false),
				Remember:  boolPtr(true),
				Scope:     stringPtr("all"),
				Transcode: boolPtr(false),
			},
			Video: PlaybackVideoConfig{
				Enabled:   boolPtr(true),
				Scope:     stringPtr("folder"),
				Transcode: boolPtr(false),
				Resume:    boolPtr(true),
			},
			Image: PlaybackImageConfig{
				Enabled: boolPtr(true),
				Scope:   stringPtr("folder"),
			},
		},
		Blacklist: BlacklistConfig{
			Extensions: []string{},
			Filenames:  []string{},
			Folders:    []string{},
			SizeRule:   "",
		},
		Security: SecurityConfig{
			IPWhitelist: []string{},
			IPBlacklist: []string{},
			PINEnabled:  false,
			PIN:         "0000",
		},
		LogLevel: "info",
		LogFile:  "",
	}
}

// ApplyDefaults applies default values to the configuration.
// It returns true if any changes were made.
func ApplyDefaults(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	changed := applyBaseDefaults(cfg)
	changed = applyFeatureDefaults(cfg) || changed
	changed = applyUIDefaults(cfg) || changed
	changed = applyPlaybackDefaults(cfg) || changed
	changed = applyBlacklistDefaults(cfg) || changed
	changed = applySecurityDefaults(cfg) || changed

	return changed
}

func applyBaseDefaults(cfg *Config) bool {
	changed := false
	if cfg.Port <= 0 {
		cfg.Port = 8099
		changed = true
	}
	if cfg.Shares == nil {
		cfg.Shares = []Share{}
		changed = true
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
		changed = true
	}
	return changed
}

func applyFeatureDefaults(cfg *Config) bool {
	changed := false
	if len(cfg.Features.SpeedOptions) == 0 &&
		!cfg.Features.Speed &&
		!cfg.Features.Quality &&
		!cfg.Features.Captions &&
		!cfg.Features.Playlist {
		cfg.Features = Features{
			Speed:        true,
			SpeedOptions: []float64{0.5, 0.75, 1, 1.25, 1.5, 2},
			Quality:      false,
			Captions:     true,
			Playlist:     true,
		}
		changed = true
	} else if len(cfg.Features.SpeedOptions) == 0 {
		cfg.Features.SpeedOptions = []float64{0.5, 0.75, 1, 1.25, 1.5, 2}
		changed = true
	}
	return changed
}

func applyUIDefaults(cfg *Config) bool {
	changed := false
	if cfg.UI.DefaultTab == nil {
		v := "video"
		cfg.UI.DefaultTab = &v
		changed = true
	}
	if cfg.UI.ShowOthers == nil {
		v := false
		cfg.UI.ShowOthers = &v
		changed = true
	}
	return changed
}

func applyPlaybackDefaults(cfg *Config) bool {
	changed := false
	changed = setDefaultBool(&cfg.Playback.Audio.Enabled, true) || changed
	changed = setDefaultBool(&cfg.Playback.Audio.Shuffle, false) || changed
	changed = setDefaultBool(&cfg.Playback.Audio.Remember, true) || changed
	changed = setDefaultString(&cfg.Playback.Audio.Scope, "all") || changed
	changed = setDefaultBool(&cfg.Playback.Audio.Transcode, false) || changed

	changed = setDefaultBool(&cfg.Playback.Video.Enabled, true) || changed
	changed = setDefaultString(&cfg.Playback.Video.Scope, "folder") || changed
	changed = setDefaultBool(&cfg.Playback.Video.Transcode, false) || changed
	changed = setDefaultBool(&cfg.Playback.Video.Resume, true) || changed

	changed = setDefaultBool(&cfg.Playback.Image.Enabled, true) || changed
	changed = setDefaultString(&cfg.Playback.Image.Scope, "folder") || changed
	return changed
}

func setDefaultBool(dst **bool, v bool) bool {
	if *dst != nil {
		return false
	}
	*dst = boolPtr(v)
	return true
}

func setDefaultString(dst **string, v string) bool {
	if *dst != nil {
		return false
	}
	*dst = stringPtr(v)
	return true
}

func applyBlacklistDefaults(cfg *Config) bool {
	changed := false
	if cfg.Blacklist.Extensions == nil {
		cfg.Blacklist.Extensions = []string{}
		changed = true
	}
	if cfg.Blacklist.Filenames == nil {
		cfg.Blacklist.Filenames = []string{}
		changed = true
	}
	if cfg.Blacklist.Folders == nil {
		cfg.Blacklist.Folders = []string{}
		changed = true
	}
	return changed
}

func applySecurityDefaults(cfg *Config) bool {
	changed := false
	if cfg.Security.IPWhitelist == nil {
		cfg.Security.IPWhitelist = []string{}
		changed = true
	}
	if cfg.Security.IPBlacklist == nil {
		cfg.Security.IPBlacklist = []string{}
		changed = true
	}
	if cfg.Security.PIN == "" {
		cfg.Security.PIN = "0000"
		changed = true
	}
	return changed
}
