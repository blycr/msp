package types

import (
	"msp/internal/config"
	"time"
)

type Subtitle struct {
	ID      string `json:"id" gorm:"column:id"`
	Label   string `json:"label" gorm:"column:label"`
	Lang    string `json:"lang" gorm:"column:lang"`
	Src     string `json:"src" gorm:"column:src"`
	Default bool   `json:"default,omitempty" gorm:"column:default"`
}

type MediaItem struct {
	ID         string     `json:"id" gorm:"primaryKey"`
	Path       string     `json:"-" gorm:"uniqueIndex;not null"`
	Name       string     `json:"name"`
	Ext        string     `json:"ext"`
	Kind       string     `json:"kind" gorm:"index:idx_kind;index:idx_scan_kind"`
	ShareLabel string     `json:"shareLabel" gorm:"index:idx_share_label;index:idx_scan_share_label"`
	Size       int64      `json:"size"`
	ModTime    int64      `json:"modTime"`
	Subtitles  []Subtitle `json:"subtitles,omitempty" gorm:"serializer:json"`
	CoverID    string     `json:"coverId,omitempty" gorm:"column:audio_cover"`
	LyricsID   string     `json:"lyricsId,omitempty" gorm:"column:audio_lyrics"`
	ScanID     int64      `json:"-" gorm:"index:idx_scan_id;index:idx_scan_kind;index:idx_scan_share_label"`
	ShareRoot  string     `json:"-"`
	CreatedAt  time.Time  `json:"-"`
	UpdatedAt  time.Time  `json:"-"`
}

type MediaScan struct {
	CacheKey  string    `gorm:"primaryKey"`
	ScanID    int64     `gorm:"not null"`
	BuiltAt   int64     `gorm:"not null"`
	Complete  bool      `gorm:"not null"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type UserPref struct {
	Key       string `gorm:"primaryKey"`
	Value     string
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type PlaybackProgress struct {
	MediaID   string    `json:"mediaId" gorm:"primaryKey"`
	Time      float64   `json:"time" gorm:"not null"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

type MediaResponse struct {
	Shares      []config.Share `json:"shares"`
	Videos      []MediaItem    `json:"videos"`
	Audios      []MediaItem    `json:"audios"`
	Images      []MediaItem    `json:"images"`
	Others      []MediaItem    `json:"others"`
	VideosTotal int            `json:"videosTotal,omitempty"`
	AudiosTotal int            `json:"audiosTotal,omitempty"`
	ImagesTotal int            `json:"imagesTotal,omitempty"`
	OthersTotal int            `json:"othersTotal,omitempty"`
	Limited     bool           `json:"limited,omitempty"`
	Scanning    bool           `json:"scanning,omitempty"`
}

type ConfigResponse struct {
	Config  interface{} `json:"config"`
	LanIPs  []string    `json:"lanIPs"`
	Urls    []string    `json:"urls"`
	NowUnix int64       `json:"nowUnix"`
	Error   *ApiError   `json:"error,omitempty"`
}

type ApiError struct {
	Message string `json:"message"`
}

type SharesOpRequest struct {
	Op    string `json:"op"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type SharesOpResponse struct {
	Config interface{} `json:"config"`
	Error  *ApiError   `json:"error,omitempty"`
}

type ProbeResponse struct {
	Container string     `json:"container"`
	Video     string     `json:"video,omitempty"`
	Audio     string     `json:"audio,omitempty"`
	Subtitles []Subtitle `json:"subtitles,omitempty"`
	Error     *ApiError  `json:"error,omitempty"`
}

type PrefsResponse struct {
	Prefs map[string]string `json:"prefs"`
	Error *ApiError         `json:"error,omitempty"`
}

type PrefsUpdateRequest struct {
	Prefs map[string]string `json:"prefs"`
}

type LogRequest struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}
