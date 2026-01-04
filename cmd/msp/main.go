package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	webassets "msp/web"
)

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

type Config struct {
	Port      int             `json:"port"`
	Shares    []Share         `json:"shares"`
	Features  Features        `json:"features"`
	UI        UIConfig        `json:"ui"`
	Playback  PlaybackConfig  `json:"playback"`
	Blacklist BlacklistConfig `json:"blacklist"`
	LogLevel  string          `json:"logLevel"`
	LogFile   string          `json:"logFile"`
}

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
	Shares []Share     `json:"shares"`
	Videos []MediaItem `json:"videos"`
	Audios []MediaItem `json:"audios"`
	Images []MediaItem `json:"images"`
	Others []MediaItem `json:"others"`
}

type ConfigResponse struct {
	Config  Config    `json:"config"`
	LanIPs  []string  `json:"lanIPs"`
	Urls    []string  `json:"urls"`
	NowUnix int64     `json:"nowUnix"`
	Error   *ApiError `json:"error,omitempty"`
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
	Config Config    `json:"config"`
	Error  *ApiError `json:"error,omitempty"`
}

type ProbeResponse struct {
	Container string    `json:"container"`
	Video     string    `json:"video,omitempty"`
	Audio     string    `json:"audio,omitempty"`
	Error     *ApiError `json:"error,omitempty"`
}

type Server struct {
	mu      sync.RWMutex
	cfg     Config
	cfgPath string

	mediaMu       sync.Mutex
	mediaCond     *sync.Cond
	mediaKey      string
	mediaBuiltAt  time.Time
	mediaTTL      time.Duration
	mediaResp     MediaResponse
	mediaETag     string
	mediaBuilding bool

	seenIPs sync.Map
}

func main() {
	debug.SetGCPercent(50) // Aggressive GC to keep memory low
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	s := &Server{cfgPath: filepath.Join(mustExeDir(), "config.json")}
	s.mediaTTL = 2 * time.Minute
	s.mediaCond = sync.NewCond(&s.mediaMu)
	if err := s.loadOrInitConfig(); err != nil {
		log.Fatal(err)
	}

	s.setupLogger()

	webRoot, err := fs.Sub(webassets.FS, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())

	mux.Handle("/api/config", http.HandlerFunc(s.handleConfig))
	mux.Handle("/api/shares", http.HandlerFunc(s.handleShares))
	mux.Handle("/api/media", http.HandlerFunc(s.handleMedia))
	mux.Handle("/api/stream", http.HandlerFunc(s.handleStream))
	mux.Handle("/api/subtitle", http.HandlerFunc(s.handleSubtitle))
	mux.Handle("/api/probe", http.HandlerFunc(s.handleProbe))
	mux.Handle("/api/ip", http.HandlerFunc(s.handleIP))

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedWeb(w, r, webRoot)
	}))

	port := s.getPort()
	addr := ":" + itoa(port)

	ips := getLanIPv4s()
	urls := make([]string, 0, 2+len(ips))
	urls = append(urls, "http://127.0.0.1:"+itoa(port)+"/")
	for _, ip := range ips {
		urls = append(urls, "http://"+ip+":"+itoa(port)+"/")
	}

	log.Println("配置文件:", s.cfgPath)
	fmt.Println("配置文件:", s.cfgPath)
	for _, u := range urls {
		log.Println("访问:", u)
		fmt.Println("访问:", u)
	}

	h := s.withLog(withGzip(mux))
	server := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (s *Server) getPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Port <= 0 {
		return 8099
	}
	return s.cfg.Port
}

func (s *Server) loadOrInitConfig() error {
	b, err := os.ReadFile(s.cfgPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
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
		cfg := Config{
			Port:   8099,
			Shares: []Share{},
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
		}
		s.mu.Lock()
		s.cfg = cfg
		s.mu.Unlock()
		return s.saveConfigLocked()
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	cfg.Shares = normalizeShares(cfg.Shares)
	changed := applyConfigDefaults(&cfg)

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	if changed {
		return s.saveConfigLocked()
	}
	return nil
}

func applyConfigDefaults(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	changed := false

	if cfg.Port <= 0 {
		cfg.Port = 8099
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

	return changed
}

func (s *Server) saveConfigLocked() error {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.cfgPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.cfgPath)
}

func (s *Server) setupLogger() {
	s.mu.Lock()
	if s.cfg.LogFile == "" {
		s.cfg.LogFile = filepath.Join(mustExeDir(), "msp.log")
	}
	logFile := s.cfg.LogFile
	s.mu.Unlock()

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	log.SetOutput(f)
}

func (s *Server) logRequest(r *http.Request, status int, start time.Time) {
	if status == 0 {
		status = http.StatusOK
	}
	ua := strings.TrimSpace(r.UserAgent())
	duration := time.Since(start).Milliseconds()

	msg := fmt.Sprintf("%s %s status=%d ua=%s ms=%d", r.Method, r.URL.Path, status, ua, duration)

	s.mu.RLock()
	level := strings.ToLower(s.cfg.LogLevel)
	s.mu.RUnlock()

	if level != "silent" && level != "none" {
		log.Println(msg)
	}

	isSevere := status >= 500

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	isNewDevice := false
	if ip != "" && ip != "127.0.0.1" && ip != "::1" {
		if _, seen := s.seenIPs.Load(ip); !seen {
			s.seenIPs.Store(ip, true)
			isNewDevice = true
		}
	}

	if isSevere || isNewDevice {
		ts := time.Now().Format("2006/01/02 15:04:05.000000")
		prefix := ""
		if isSevere {
			prefix = "[ERROR] "
		}
		if isNewDevice {
			prefix += "[NEW DEVICE] "
		}
		fmt.Fprintf(os.Stderr, "%s %s%s\n", ts, prefix, msg)
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ips := getLanIPv4s()
		port := s.getPort()
		urls := make([]string, 0, 2+len(ips))
		urls = append(urls, "http://127.0.0.1:"+itoa(port)+"/")
		for _, ip := range ips {
			urls = append(urls, "http://"+ip+":"+itoa(port)+"/")
		}

		s.mu.RLock()
		cfg := s.cfg
		s.mu.RUnlock()

		writeJSON(w, http.StatusOK, ConfigResponse{
			Config:  cfg,
			LanIPs:  ips,
			Urls:    urls,
			NowUnix: time.Now().Unix(),
		})
	case http.MethodPost:
		var cfg Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, ConfigResponse{Error: &ApiError{Message: "JSON 解析失败"}})
			return
		}
		applyConfigDefaults(&cfg)
		cfg.Shares = normalizeShares(cfg.Shares)

		validShares := make([]Share, 0, len(cfg.Shares))
		for _, sh := range cfg.Shares {
			if sh.Path == "" {
				continue
			}
			p := normalizeWinPath(sh.Path)
			if ok := isExistingDir(p); !ok {
				continue
			}
			sh.Path = p
			if sh.Label == "" {
				sh.Label = filepath.Base(p)
			}
			validShares = append(validShares, sh)
		}
		cfg.Shares = dedupeShares(validShares)

		s.mu.Lock()
		s.cfg = cfg
		s.mu.Unlock()

		if err := s.saveConfigLocked(); err != nil {
			writeJSON(w, http.StatusInternalServerError, ConfigResponse{Error: &ApiError{Message: "写入配置失败"}})
			return
		}
		s.invalidateMediaCache()
		writeJSON(w, http.StatusOK, ConfigResponse{Config: cfg})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req SharesOpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SharesOpResponse{Error: &ApiError{Message: "JSON 解析失败"}})
		return
	}

	op := strings.ToLower(strings.TrimSpace(req.Op))
	p := normalizeWinPath(req.Path)
	label := strings.TrimSpace(req.Label)
	if label == "" && p != "" {
		label = filepath.Base(p)
	}

	s.mu.Lock()
	cfg := s.cfg
	switch op {
	case "add":
		if p == "" || !isExistingDir(p) {
			s.mu.Unlock()
			writeJSON(w, http.StatusBadRequest, SharesOpResponse{Error: &ApiError{Message: "目录不存在或不可访问"}})
			return
		}
		cfg.Shares = append(cfg.Shares, Share{Label: label, Path: p})
		cfg.Shares = normalizeShares(cfg.Shares)
		cfg.Shares = dedupeShares(cfg.Shares)
		s.cfg = cfg
	case "remove":
		if p == "" {
			s.mu.Unlock()
			writeJSON(w, http.StatusBadRequest, SharesOpResponse{Error: &ApiError{Message: "缺少 Path"}})
			return
		}
		out := make([]Share, 0, len(cfg.Shares))
		for _, sh := range cfg.Shares {
			if !samePathWin(sh.Path, p) {
				out = append(out, sh)
			}
		}
		cfg.Shares = out
		s.cfg = cfg
	default:
		s.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, SharesOpResponse{Error: &ApiError{Message: "不支持的 op（add/remove）"}})
		return
	}
	s.mu.Unlock()

	if err := s.saveConfigLocked(); err != nil {
		writeJSON(w, http.StatusInternalServerError, SharesOpResponse{Error: &ApiError{Message: "写入配置失败"}})
		return
	}

	s.invalidateMediaCache()
	writeJSON(w, http.StatusOK, SharesOpResponse{Config: cfg})
}

func (s *Server) handleIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lanIPs": getLanIPv4s(),
	})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	shares := append([]Share(nil), s.cfg.Shares...)
	blacklist := s.cfg.Blacklist
	s.mu.RUnlock()

	refresh := r.URL.Query().Get("refresh") == "1"
	resp, etag := s.getOrBuildMediaCache(shares, blacklist, refresh)
	if etag != "" {
		w.Header().Set("ETag", etag)
		if !refresh && strings.TrimSpace(r.Header.Get("If-None-Match")) == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) invalidateMediaCache() {
	s.mediaMu.Lock()
	s.mediaKey = ""
	s.mediaETag = ""
	s.mediaBuiltAt = time.Time{}
	s.mediaResp = MediaResponse{}
	s.mediaMu.Unlock()
}

func (s *Server) getOrBuildMediaCache(shares []Share, blacklist BlacklistConfig, refresh bool) (MediaResponse, string) {
	key := sharesCacheKey(shares)

	for {
		s.mediaMu.Lock()
		valid := s.mediaKey == key && !s.mediaBuiltAt.IsZero() && time.Since(s.mediaBuiltAt) < s.mediaTTL
		if valid && !refresh {
			resp := s.mediaResp
			etag := s.mediaETag
			s.mediaMu.Unlock()
			return resp, etag
		}
		if s.mediaBuilding {
			s.mediaCond.Wait()
			s.mediaMu.Unlock()
			continue
		}
		s.mediaBuilding = true
		s.mediaMu.Unlock()

		resp := buildMediaResponse(shares, blacklist)
		builtAt := time.Now()
		etag := weakETag(key, builtAt)

		s.mediaMu.Lock()
		s.mediaResp = resp
		s.mediaKey = key
		s.mediaBuiltAt = builtAt
		s.mediaETag = etag
		s.mediaBuilding = false
		s.mediaCond.Broadcast()
		s.mediaMu.Unlock()
		return resp, etag
	}
}

func sharesCacheKey(shares []Share) string {
	s := append([]Share(nil), shares...)
	for i := range s {
		s[i].Path = normalizeWinPath(s[i].Path)
	}
	sort.Slice(s, func(i, j int) bool {
		return strings.ToLower(s[i].Path) < strings.ToLower(s[j].Path)
	})
	var b strings.Builder
	for _, sh := range s {
		b.WriteString(strings.ToLower(sh.Path))
		b.WriteByte('|')
		b.WriteString(strings.TrimSpace(sh.Label))
		b.WriteByte('\n')
	}
	return b.String()
}

func weakETag(key string, builtAt time.Time) string {
	h := fnv.New64a()
	h.Write([]byte(key))
	var t [8]byte
	n := uint64(builtAt.UnixNano())
	for i := 0; i < 8; i++ {
		t[i] = byte(n)
		n >>= 8
	}
	h.Write(t[:])
	return `W/"` + u64Base36(h.Sum64()) + `"`
}

func buildMediaResponse(shares []Share, blacklist BlacklistConfig) MediaResponse {
	resp := MediaResponse{
		Shares: shares,
		Videos: []MediaItem{},
		Audios: []MediaItem{},
		Images: []MediaItem{},
		Others: []MediaItem{},
	}

	type gathered struct {
		item MediaItem
	}
	var items []gathered

	const maxItems = 8000
	for _, sh := range shares {
		root := normalizeWinPath(sh.Path)
		if root == "" || !isExistingDir(root) {
			continue
		}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if len(items) >= maxItems {
				return fs.SkipAll
			}
			if d.IsDir() {
				name := d.Name()
				if name == "" {
					return nil
				}
				if strings.HasPrefix(name, ".") {
					return fs.SkipDir
				}
				if isBlockedString(blacklist.Folders, name) {
					return fs.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if ext == "" {
				return nil
			}
			if isBlockedString(blacklist.Extensions, ext) {
				return nil
			}
			if isBlockedString(blacklist.Filenames, d.Name()) {
				return nil
			}
			if isSubtitleExt(ext) || isLyricsExt(ext) {
				return nil
			}

			fi, statErr := d.Info()
			if statErr != nil {
				return nil
			}

			if isBlockedSize(fi.Size(), blacklist.SizeRule) {
				return nil
			}

			kind := classifyExt(ext)
			item := MediaItem{
				ID:         encodeID(p),
				Name:       d.Name(),
				Ext:        ext,
				Kind:       kind,
				ShareLabel: sh.Label,
				Size:       fi.Size(),
				ModTime:    fi.ModTime().Unix(),
			}

			if kind == "video" {
				item.Subtitles = findSidecarSubtitles(p)
			}
			if kind == "audio" {
				cover, lyrics := findAudioSidecars(p)
				if cover != "" {
					item.CoverID = encodeID(cover)
				}
				if lyrics != "" {
					item.LyricsID = encodeID(lyrics)
				}
			}

			items = append(items, gathered{item: item})
			return nil
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].item.Kind != items[j].item.Kind {
			return items[i].item.Kind < items[j].item.Kind
		}
		if items[i].item.ShareLabel != items[j].item.ShareLabel {
			return items[i].item.ShareLabel < items[j].item.ShareLabel
		}
		return strings.ToLower(items[i].item.Name) < strings.ToLower(items[j].item.Name)
	})

	for _, g := range items {
		switch g.item.Kind {
		case "video":
			resp.Videos = append(resp.Videos, g.item)
		case "audio":
			resp.Audios = append(resp.Audios, g.item)
		case "image":
			resp.Images = append(resp.Images, g.item)
		default:
			resp.Others = append(resp.Others, g.item)
		}
	}
	return resp
}

func isBlockedString(list []string, target string) bool {
	targetLower := strings.ToLower(target)
	for _, rule := range list {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		// Regex support: /pattern/
		if strings.HasPrefix(rule, "/") && strings.HasSuffix(rule, "/") && len(rule) > 2 {
			pattern := rule[1 : len(rule)-1]
			// Try to match against the original string (case sensitive?) or lower?
			// Let's match against original for maximum flexibility, user can use (?i) for case insensitive
			if matched, _ := regexp.MatchString(pattern, target); matched {
				return true
			}
			continue
		}

		// Exact match (case insensitive)
		if strings.EqualFold(rule, target) {
			return true
		}
		// Also match lower case for safety
		if strings.EqualFold(rule, targetLower) {
			return true
		}
	}
	return false
}

func isBlockedSize(size int64, rule string) bool {
	rule = strings.TrimSpace(strings.ToUpper(rule))
	if rule == "" {
		return false
	}

	// Range: "100KB-200MB"
	if parts := strings.Split(rule, "-"); len(parts) == 2 {
		min := parseSize(parts[0])
		max := parseSize(parts[1])
		if min >= 0 && max > 0 {
			return size >= min && size <= max
		}
	}

	// Greater/Less
	if strings.HasPrefix(rule, ">=") {
		val := parseSize(strings.TrimPrefix(rule, ">="))
		return size >= val
	}
	if strings.HasPrefix(rule, "<=") {
		val := parseSize(strings.TrimPrefix(rule, "<="))
		return size <= val
	}
	if strings.HasPrefix(rule, ">") {
		val := parseSize(strings.TrimPrefix(rule, ">"))
		return size > val
	}
	if strings.HasPrefix(rule, "<") {
		val := parseSize(strings.TrimPrefix(rule, "<"))
		return size < val
	}

	return false
}

func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	scale := int64(1)
	if strings.HasSuffix(s, "TB") {
		scale = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "GB") {
		scale = 1024 * 1024 * 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "MB") {
		scale = 1024 * 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "KB") {
		scale = 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "B") {
		s = s[:len(s)-1]
	}

	val, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return int64(val * float64(scale))
}

func isLyricsExt(ext string) bool {
	switch ext {
	case ".lrc":
		return true
	default:
		return false
	}
}

func findAudioSidecars(mediaAbs string) (coverAbs string, lyricsAbs string) {
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))

	lyrics := filepath.Join(dir, base+".lrc")
	if st, err := os.Stat(lyrics); err == nil && !st.IsDir() {
		lyricsAbs = lyrics
	}

	candidates := []string{
		filepath.Join(dir, base+".jpg"),
		filepath.Join(dir, base+".jpeg"),
		filepath.Join(dir, base+".png"),
		filepath.Join(dir, base+".webp"),
		filepath.Join(dir, "cover.jpg"),
		filepath.Join(dir, "folder.jpg"),
		filepath.Join(dir, "front.jpg"),
		filepath.Join(dir, "album.jpg"),
		filepath.Join(dir, "albumart.jpg"),
	}
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err == nil && !st.IsDir() {
			coverAbs = p
			break
		}
	}
	return coverAbs, lyricsAbs
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	target, err := decodeID(id)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	target = normalizeWinPath(target)

	s.mu.RLock()
	shares := append([]Share(nil), s.cfg.Shares...)
	s.mu.RUnlock()

	if !isAllowedFile(target, shares) {
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}

	f, err := os.Open(target)
	if err != nil {
		http.Error(w, "open failed", http.StatusNotFound)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(st.Name()))
	var ct string
	switch ext {
	case ".mp4", ".m4v":
		ct = "video/mp4"
	case ".mkv":
		ct = "video/x-matroska"
	case ".webm":
		ct = "video/webm"
	case ".avi":
		ct = "video/x-msvideo"
	case ".mov":
		ct = "video/quicktime"
	case ".ts":
		ct = "video/mp2t"
	case ".vtt":
		ct = "text/vtt; charset=utf-8"
	case ".srt", ".lrc":
		ct = "text/plain; charset=utf-8"
	}

	if ct == "" {
		ct = mime.TypeByExtension(ext)
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=0")

	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ProbeResponse{Error: &ApiError{Message: "missing id"}})
		return
	}

	target, err := decodeID(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ProbeResponse{Error: &ApiError{Message: "bad id"}})
		return
	}
	target = normalizeWinPath(target)

	s.mu.RLock()
	shares := append([]Share(nil), s.cfg.Shares...)
	s.mu.RUnlock()

	if !isAllowedFile(target, shares) {
		writeJSON(w, http.StatusForbidden, ProbeResponse{Error: &ApiError{Message: "not allowed"}})
		return
	}

	ext := strings.ToLower(filepath.Ext(target))
	video, audio := sniffContainerCodecs(target, ext)
	writeJSON(w, http.StatusOK, ProbeResponse{
		Container: strings.TrimPrefix(ext, "."),
		Video:     video,
		Audio:     audio,
	})
}

func (s *Server) handleSubtitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	target, err := decodeID(id)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	target = normalizeWinPath(target)

	s.mu.RLock()
	shares := append([]Share(nil), s.cfg.Shares...)
	s.mu.RUnlock()

	if !isAllowedFile(target, shares) {
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}

	f, err := os.Open(target)
	if err != nil {
		http.Error(w, "open failed", http.StatusNotFound)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(st.Name()))
	switch ext {
	case ".vtt":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Cache-Control", "private, max-age=0")
		http.ServeContent(w, r, st.Name(), st.ModTime(), f)
		return
	case ".srt":
		b, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "read failed", http.StatusInternalServerError)
			return
		}
		out := srtToVtt(b)
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Cache-Control", "private, max-age=0")
		http.ServeContent(w, r, strings.TrimSuffix(st.Name(), ext)+".vtt", st.ModTime(), bytes.NewReader(out))
		return
	default:
		http.Error(w, "unsupported subtitle format", http.StatusBadRequest)
		return
	}
}

func sniffContainerCodecs(fileAbs string, ext string) (string, string) {
	f, err := os.Open(fileAbs)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	const max = 2 << 20
	head, err := io.ReadAll(io.LimitReader(f, max))
	if err != nil || len(head) == 0 {
		return "", ""
	}
	b := head
	if st, statErr := f.Stat(); statErr == nil {
		size := st.Size()
		if size > max {
			tailSize := int64(max)
			if size < tailSize {
				tailSize = size
			}
			tail := make([]byte, tailSize)
			_, _ = f.ReadAt(tail, size-tailSize)
			b = append(head, tail...)
		}
	}

	video := ""
	audioParts := make([]string, 0, 2)

	has := func(s string) bool {
		return bytes.Contains(b, []byte(s))
	}

	if ext == ".mkv" {
		switch {
		case has("V_MPEGH/ISO/HEVC"):
			video = "H.265/HEVC"
		case has("V_MPEG4/ISO/AVC"):
			video = "H.264/AVC"
		case has("V_AV1"):
			video = "AV1"
		case has("V_VP9"):
			video = "VP9"
		}
		switch {
		case has("A_EAC3"):
			audioParts = append(audioParts, "E-AC-3")
		case has("A_AC3"):
			audioParts = append(audioParts, "AC-3")
		case has("A_OPUS"):
			audioParts = append(audioParts, "Opus")
		case has("A_AAC"):
			audioParts = append(audioParts, "AAC")
		case has("A_VORBIS"):
			audioParts = append(audioParts, "Vorbis")
		case has("A_FLAC"):
			audioParts = append(audioParts, "FLAC")
		case has("A_DTS"):
			audioParts = append(audioParts, "DTS")
		case has("A_TRUEHD"):
			audioParts = append(audioParts, "TrueHD")
		}
		return video, strings.Join(audioParts, " + ")
	}

	if ext == ".mp4" || ext == ".m4v" || ext == ".mov" {
		switch {
		case has("hvc1") || has("hev1"):
			video = "H.265/HEVC"
		case has("avc1"):
			video = "H.264/AVC"
		case has("av01"):
			video = "AV1"
		case has("vp09"):
			video = "VP9"
		}
		switch {
		case has("ec-3"):
			audioParts = append(audioParts, "E-AC-3")
		case has("ac-3"):
			audioParts = append(audioParts, "AC-3")
		case has("mp4a"):
			audioParts = append(audioParts, "AAC/MP4A")
		case has("opus"):
			audioParts = append(audioParts, "Opus")
		}
		return video, strings.Join(audioParts, " + ")
	}

	return "", ""
}

func normalizeWinPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, `"`, "")
	p = filepath.FromSlash(p)
	p = filepath.Clean(p)
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	return p
}

func normalizeShares(in []Share) []Share {
	out := make([]Share, 0, len(in))
	for _, sh := range in {
		p := normalizeWinPath(sh.Path)
		if p == "" {
			continue
		}
		lbl := strings.TrimSpace(sh.Label)
		if lbl == "" {
			lbl = filepath.Base(p)
		}
		out = append(out, Share{Label: lbl, Path: p})
	}
	return out
}

func dedupeShares(in []Share) []Share {
	out := make([]Share, 0, len(in))
	seen := map[string]bool{}
	for _, sh := range in {
		key := strings.ToLower(sh.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, sh)
	}
	return out
}

func samePathWin(a, b string) bool {
	return strings.EqualFold(normalizeWinPath(a), normalizeWinPath(b))
}

func isExistingDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isAllowedFile(fileAbs string, shares []Share) bool {
	if fileAbs == "" {
		return false
	}
	f, err := filepath.Abs(fileAbs)
	if err != nil {
		return false
	}
	f = filepath.Clean(f)

	for _, sh := range shares {
		root := normalizeWinPath(sh.Path)
		if root == "" {
			continue
		}
		if withinWinRoot(root, f) {
			st, err := os.Stat(f)
			return err == nil && !st.IsDir()
		}
	}
	return false
}

func withinWinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if strings.EqualFold(root, target) {
		return true
	}
	rs := root
	if !strings.HasSuffix(rs, string(os.PathSeparator)) {
		rs += string(os.PathSeparator)
	}
	return strings.HasPrefix(strings.ToLower(target), strings.ToLower(rs))
}

func encodeID(absPath string) string {
	b := []byte(absPath)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("empty")
	}
	return string(b), nil
}

func isSubtitleExt(ext string) bool {
	switch ext {
	case ".vtt", ".srt":
		return true
	default:
		return false
	}
}

func findSidecarSubtitles(mediaAbs string) []Subtitle {
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	baseLower := strings.ToLower(base)
	var out []Subtitle

	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		ext := strings.ToLower(filepath.Ext(low))
		if ext != ".vtt" && ext != ".srt" {
			continue
		}
		stem := strings.TrimSuffix(low, ext)

		token := ""
		if stem == baseLower {
			token = ""
		} else if strings.HasPrefix(stem, baseLower+".") {
			token = strings.TrimPrefix(stem, baseLower+".")
		} else {
			continue
		}

		abs := filepath.Join(dir, name)
		id := encodeID(abs)
		src := "/api/stream?id=" + id
		if ext == ".srt" {
			src = "/api/subtitle?id=" + id
		}

		lang := "zh"
		label := "字幕"
		if token != "" {
			lang = token
			label = subtitleLabel(token)
		}

		out = append(out, Subtitle{
			ID:    id,
			Label: label,
			Lang:  lang,
			Src:   src,
		})
	}

	if len(out) == 0 {
		return nil
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Lang == "zh" && out[j].Lang != "zh" {
			return true
		}
		if out[i].Lang != "zh" && out[j].Lang == "zh" {
			return false
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})

	out[0].Default = true
	return out
}

func subtitleLabel(token string) string {
	t := strings.ToLower(strings.TrimSpace(token))
	switch t {
	case "zh", "zh-cn", "zh-hans":
		return "中文"
	case "zh-tw", "zh-hant":
		return "繁體"
	case "en", "en-us", "en-gb":
		return "English"
	case "ja", "jp":
		return "日本語"
	case "ko":
		return "한국어"
	case "fr":
		return "Français"
	case "de":
		return "Deutsch"
	case "es":
		return "Español"
	case "ru":
		return "Русский"
	default:
		return token
	}
}

func srtToVtt(in []byte) []byte {
	in = bytes.TrimPrefix(in, []byte{0xEF, 0xBB, 0xBF})
	s := string(in)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")

	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out.WriteString("\n")
			continue
		}
		if isAllDigits(trimmed) {
			continue
		}
		if strings.Contains(line, "-->") {
			out.WriteString(strings.ReplaceAll(line, ",", "."))
			out.WriteString("\n")
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return []byte(out.String())
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func classifyExt(ext string) string {
	switch ext {
	case ".mp4", ".webm", ".mkv", ".mov", ".avi", ".m4v":
		return "video"
	case ".mp3", ".aac", ".wav", ".flac", ".m4a", ".ogg", ".opus":
		return "audio"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return "image"
	default:
		return "other"
	}
}

func getLanIPv4s() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}
			if isPrivateIPv4(ip4) {
				ips = append(ips, ip4.String())
			}
		}
	}
	sort.Strings(ips)
	ips = dedupeStrings(ips)
	return ips
}

func isPrivateIPv4(ip net.IP) bool {
	if ip == nil || len(ip) != 4 {
		return false
	}
	switch {
	case ip[0] == 10:
		return true
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return true
	case ip[0] == 192 && ip[1] == 168:
		return true
	default:
		return false
	}
}

func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func serveEmbeddedWeb(w http.ResponseWriter, r *http.Request, webFS fs.FS) {
	p := path.Clean("/" + r.URL.Path)
	if p == "/" || p == "/index.html" {
		serveEmbeddedFSFile(w, r, webFS, "index.html", "text/html; charset=utf-8", "no-store")
		return
	}
	if strings.HasPrefix(p, "/api/") {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(p, "/")
	ext := strings.ToLower(filepath.Ext(name))
	cache := "no-store"
	switch ext {
	case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".woff2", ".ttf":
		cache = "no-cache"
	}
	serveEmbeddedFSFile(w, r, webFS, name, "", cache)
}

func serveEmbeddedFSFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string, contentType string, cacheControl string) {
	f, err := fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}

	ext := strings.ToLower(filepath.Ext(st.Name()))
	ct := contentType
	if ct == "" {
		ct = mime.TypeByExtension(ext)
		if ct == "" {
			switch ext {
			case ".vtt":
				ct = "text/vtt; charset=utf-8"
			case ".lrc", ".srt":
				ct = "text/plain; charset=utf-8"
			default:
				ct = "application/octet-stream"
			}
		}
	}
	w.Header().Set("Content-Type", ct)
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	http.ServeContent(w, r, st.Name(), st.ModTime(), readSeeker{f})
}

type readSeeker struct {
	f fs.File
}

func (r readSeeker) Read(p []byte) (int, error) {
	return r.f.Read(p)
}

func (r readSeeker) Seek(offset int64, whence int) (int64, error) {
	if s, ok := r.f.(io.Seeker); ok {
		return s.Seek(offset, whence)
	}
	return 0, errors.New("seek not supported")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gw *gzip.Writer
}

func (g gzipResponseWriter) Write(p []byte) (int, error) {
	return g.gw.Write(p)
}

func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ae := r.Header.Get("Accept-Encoding")
		if !strings.Contains(ae, "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/stream" || r.URL.Path == "/api/subtitle" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzip.NewWriter(w)
		defer gw.Close()
		next.ServeHTTP(gzipResponseWriter{ResponseWriter: w, gw: gw}, r)
	})
}

func (s *Server) withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		s.logRequest(r, sw.status, start)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func mustExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func u64Base36(u uint64) string {
	if u == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var b [32]byte
	pos := len(b)
	for u > 0 {
		pos--
		b[pos] = digits[u%36]
		u /= 36
	}
	return string(b[pos:])
}
