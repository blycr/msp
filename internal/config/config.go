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
	DefaultTab *string `json:"defaultTab,omitempty"`
	ShowOthers *bool   `json:"showOthers,omitempty"`
}

type PlaybackAudioConfig struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	Shuffle  *bool   `json:"shuffle,omitempty"`
	Remember *bool   `json:"remember,omitempty"`
	Scope    *string `json:"scope,omitempty"`
}

type PlaybackVideoConfig struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Scope   *string `json:"scope,omitempty"`
}

type PlaybackImageConfig struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Scope   *string `json:"scope,omitempty"`
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

// Default configuration values
func Default() Config {
	defTab := "video"
	showOthers := false
	audioEnabled := true
	audioShuffle := false
	audioRemember := true
	audioScope := "all"
	videoEnabled := true
	videoScope := "folder"
	imageEnabled := true
	imageScope := "folder"

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
			DefaultTab: &defTab,
			ShowOthers: &showOthers,
		},
		Playback: PlaybackConfig{
			Audio: PlaybackAudioConfig{
				Enabled:  &audioEnabled,
				Shuffle:  &audioShuffle,
				Remember: &audioRemember,
				Scope:    &audioScope,
			},
			Video: PlaybackVideoConfig{
				Enabled: &videoEnabled,
				Scope:   &videoScope,
			},
			Image: PlaybackImageConfig{
				Enabled: &imageEnabled,
				Scope:   &imageScope,
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

func ApplyDefaults(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	changed := false

	if cfg.Port <= 0 {
		cfg.Port = 8099
		changed = true
	}

	if cfg.Shares == nil {
		cfg.Shares = []Share{}
		changed = true
	}

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

	if cfg.Playback.Audio.Enabled == nil {
		v := true
		cfg.Playback.Audio.Enabled = &v
		changed = true
	}
	if cfg.Playback.Audio.Shuffle == nil {
		v := false
		cfg.Playback.Audio.Shuffle = &v
		changed = true
	}
	if cfg.Playback.Audio.Remember == nil {
		v := true
		cfg.Playback.Audio.Remember = &v
		changed = true
	}
	if cfg.Playback.Audio.Scope == nil {
		v := "all"
		cfg.Playback.Audio.Scope = &v
		changed = true
	}

	if cfg.Playback.Video.Enabled == nil {
		v := true
		cfg.Playback.Video.Enabled = &v
		changed = true
	}
	if cfg.Playback.Video.Scope == nil {
		v := "folder"
		cfg.Playback.Video.Scope = &v
		changed = true
	}

	if cfg.Playback.Image.Enabled == nil {
		v := true
		cfg.Playback.Image.Enabled = &v
		changed = true
	}
	if cfg.Playback.Image.Scope == nil {
		v := "folder"
		cfg.Playback.Image.Scope = &v
		changed = true
	}

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

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
		changed = true
	}

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
