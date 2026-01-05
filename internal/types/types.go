package types

type Subtitle struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Lang    string `json:"lang"`
	Src     string `json:"src"`
	Default bool   `json:"default,omitempty"`
}

type MediaItem struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Ext        string     `json:"ext"`
	Kind       string     `json:"kind"`
	ShareLabel string     `json:"shareLabel"`
	Size       int64      `json:"size"`
	ModTime    int64      `json:"modTime"`
	Subtitles  []Subtitle `json:"subtitles,omitempty"`
	CoverID    string     `json:"coverId,omitempty"`
	LyricsID   string     `json:"lyricsId,omitempty"`
}

type MediaResponse struct {
	Shares []interface{} `json:"shares"` // Can be []config.Share if needed, but keeping interface{} avoids coupling for now
	Videos []MediaItem   `json:"videos"`
	Audios []MediaItem   `json:"audios"`
	Images []MediaItem   `json:"images"`
	Others []MediaItem   `json:"others"`
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
	Container string    `json:"container"`
	Video     string    `json:"video,omitempty"`
	Audio     string    `json:"audio,omitempty"`
	Error     *ApiError `json:"error,omitempty"`
}
