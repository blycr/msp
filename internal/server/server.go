package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"msp/internal/config"
	"msp/internal/db"
	"msp/internal/media"
	"msp/internal/types"
	"msp/internal/util"
)

type Server struct {
	mu         sync.RWMutex
	cfg        config.Config
	cfgPath    string
	cfgModTime time.Time // Last modification time of config file

	mediaCachePath string

	mediaMu       sync.Mutex
	mediaCond     *sync.Cond
	mediaKey      string
	mediaBuiltAt  time.Time
	mediaTTL      time.Duration
	mediaRespJSON []byte
	mediaETag     string
	mediaBuilding bool

	seenIPs sync.Map
	logMu   sync.Mutex
	logFile *os.File
	logCnt  int32
}

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelError = "error"
	LogLevelNone  = "none"
)

func New(cfgPath string) *Server {
	s := &Server{
		cfgPath:        cfgPath,
		mediaCachePath: cfgPath + ".media_cache.json",
	}
	s.mediaTTL = 2 * time.Minute
	s.mediaCond = sync.NewCond(&s.mediaMu)
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

	// Get file modification time
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
		s.Log(LogLevelInfo, "Config updated with default values and saved to disk")
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
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.cfgPath)
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
	return s.saveConfigLocked()
}

// WatchConfig monitors the config file for changes and reloads it automatically
func (s *Server) WatchConfig(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stat, err := os.Stat(s.cfgPath)
			if err != nil {
				continue
			}

			s.mu.RLock()
			lastModTime := s.cfgModTime
			s.mu.RUnlock()

			// Check if file has been modified
			if stat.ModTime().After(lastModTime) {
				s.Log("info", "Config file changed, reloading...")

				// Read and parse new config
				b, err := os.ReadFile(s.cfgPath)
				if err != nil {
					s.Log("error", fmt.Sprintf("Failed to read config file: %v", err))
					continue
				}

				var cfg config.Config
				if err := json.Unmarshal(b, &cfg); err != nil {
					s.Log("error", fmt.Sprintf("Failed to parse config file: %v", err))
					continue
				}

				config.ApplyDefaults(&cfg)

				// Update config
				s.mu.Lock()
				s.cfg = cfg
				s.cfgModTime = stat.ModTime()
				s.mu.Unlock()

				s.Log("info", "Config reloaded successfully")
			}
		}
	}
}

func (s *Server) SetupLogger() {
	s.mu.Lock()
	if s.cfg.LogFile == "" {
		s.cfg.LogFile = filepath.Join(util.MustExeDir(), "logs", "msp.log")
	}
	logFile := s.cfg.LogFile
	s.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logFile), 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		return
	}

	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.logFile != nil {
		s.logFile.Close()
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	s.logFile = f
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func (s *Server) Log(level string, msg string) {
	s.mu.RLock()
	cfgLevel := strings.ToLower(s.cfg.LogLevel)
	s.mu.RUnlock()

	// Level priority: error > info > debug
	shouldLog := false
	switch strings.ToLower(level) {
	case LogLevelError:
		shouldLog = cfgLevel != LogLevelNone
	case LogLevelInfo:
		shouldLog = cfgLevel == LogLevelInfo || cfgLevel == LogLevelDebug
	case LogLevelDebug:
		shouldLog = cfgLevel == LogLevelDebug
	}

	if shouldLog {
		ts := time.Now().Format("2006/01/02 15:04:05.000000")
		line := fmt.Sprintf("%s [%s] %s", ts, strings.ToUpper(level), msg)
		log.Println(line)

		// Rotate only every 100 logs or so to reduce Stat overhead
		if cnt := atomic.AddInt32(&s.logCnt, 1); cnt%100 == 0 {
			s.RotateLogIfNeeded()
		}
	}
}

func (s *Server) RotateLogIfNeeded() {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.logFile == nil {
		return
	}

	st, err := s.logFile.Stat()
	if err != nil {
		return
	}

	// 10MB limit
	if st.Size() < 10*1024*1024 {
		return
	}

	s.logFile.Close()
	s.logFile = nil

	s.mu.RLock()
	path := s.cfg.LogFile
	s.mu.RUnlock()

	oldPath := path + ".1"
	_ = os.Remove(oldPath)
	_ = os.Rename(path, oldPath)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		s.logFile = f
		log.SetOutput(f)
	}
}

func (s *Server) LogRequest(r *http.Request, status int, start time.Time) {
	if status == 0 {
		status = http.StatusOK
	}
	ua := strings.TrimSpace(r.UserAgent())
	duration := time.Since(start).Milliseconds()

	msg := fmt.Sprintf("%s %s status=%d ua=%s ms=%d", r.Method, r.URL.Path, status, ua, duration)

	level := LogLevelInfo
	if status >= 500 {
		level = LogLevelError
	}
	s.Log(level, msg)

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	if ip != "" && ip != "127.0.0.1" && ip != "::1" {
		if _, seen := s.seenIPs.Load(ip); !seen {
			s.seenIPs.Store(ip, true)
			s.Log(LogLevelInfo, fmt.Sprintf("[NEW DEVICE] %s %s", ip, msg))
		}
	}
}

func (s *Server) GetPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Port <= 0 {
		return 8099
	}
	return s.cfg.Port
}

func (s *Server) InvalidateMediaCache() {
	s.mediaMu.Lock()
	s.mediaKey = ""
	s.mediaETag = ""
	s.mediaBuiltAt = time.Time{}
	s.mediaRespJSON = nil
	s.mediaMu.Unlock()
	_ = os.Remove(s.mediaCachePath)
}

func (s *Server) GetOrBuildMediaCache(ctx context.Context, shares []config.Share, blacklist config.BlacklistConfig, refresh bool) (types.MediaResponse, string) {
	key := mediaCacheKey(shares, blacklist)

	s.mediaMu.Lock()
	// 1. Check if we have valid memory cache
	if s.mediaKey == key && !s.mediaBuiltAt.IsZero() && !refresh {
		if time.Since(s.mediaBuiltAt) >= s.mediaTTL && !s.mediaBuilding {
			s.mediaBuilding = true
			go s.rebuildMediaCache(context.Background(), key, shares, blacklist, s.cfg.MaxItems)
		}
		var r types.MediaResponse
		_ = json.Unmarshal(s.mediaRespJSON, &r)
		etag := s.mediaETag
		s.mediaMu.Unlock()
		return r, etag
	}

	// 2. If already building, return current (partial/old) data
	if s.mediaBuilding {
		var r types.MediaResponse
		_ = json.Unmarshal(s.mediaRespJSON, &r)
		r.Scanning = true
		etag := s.mediaETag
		s.mediaMu.Unlock()
		return r, etag
	}

	// 3. If refresh requested, trigger in background and return what we have
	if refresh {
		s.mediaBuilding = true
		go s.rebuildMediaCache(context.Background(), key, shares, blacklist, s.cfg.MaxItems)
		var r types.MediaResponse
		_ = json.Unmarshal(s.mediaRespJSON, &r)
		r.Scanning = true
		etag := s.mediaETag
		s.mediaMu.Unlock()
		return r, etag
	}

	// 4. Try DB if not building and key changed or expired
	if s.mediaKey != key {
		s.mediaMu.Unlock()
		if resp, builtAt, ok, _ := media.LoadMediaFromDB(ctx, key, shares); ok && !builtAt.IsZero() {
			etag := weakETag(key, builtAt)
			s.mediaMu.Lock()
			s.mediaRespJSON, _ = json.Marshal(resp)
			s.mediaKey = key
			s.mediaBuiltAt = builtAt
			s.mediaETag = etag
			s.mediaMu.Unlock()
			return resp, etag
		}
		s.mediaMu.Lock()
	}

	// 4. Need to build
	s.mediaBuilding = true
	s.mediaMu.Unlock()

	var resp types.MediaResponse
	builtAt := time.Now()
	if db.DB != nil {
		r, bt, err := media.ReindexAndLoadMedia(ctx, key, shares, blacklist, s.cfg.MaxItems)
		if err == nil && !bt.IsZero() {
			resp = r
			builtAt = bt
		} else {
			resp = media.BuildMediaResponse(ctx, shares, blacklist, s.cfg.MaxItems)
			builtAt = time.Now()
		}
	} else {
		resp = media.BuildMediaResponse(ctx, shares, blacklist, s.cfg.MaxItems)
	}
	etag := weakETag(key, builtAt)

	// Serialize to JSON bytes to save memory and serve faster
	b, _ := json.Marshal(resp)

	s.mediaMu.Lock()
	s.mediaRespJSON = b
	s.mediaKey = key
	s.mediaBuiltAt = builtAt
	s.mediaETag = etag
	s.mediaBuilding = false
	s.mediaCond.Broadcast()
	s.mediaMu.Unlock()

	if db.DB == nil {
		go s.saveMediaCacheToDisk(key, builtAt, etag, resp)
	}

	// Trigger GC after heavy indexing
	go debug.FreeOSMemory()

	return resp, etag
}

func (s *Server) rebuildMediaCache(ctx context.Context, key string, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) {
	var resp types.MediaResponse
	builtAt := time.Now()
	if db.DB != nil {
		r, bt, err := media.ReindexAndLoadMedia(ctx, key, shares, blacklist, maxItems)
		if err == nil && !bt.IsZero() {
			resp = r
			builtAt = bt
		} else {
			resp = media.BuildMediaResponse(ctx, shares, blacklist, maxItems)
			builtAt = time.Now()
		}
	} else {
		resp = media.BuildMediaResponse(ctx, shares, blacklist, maxItems)
	}
	etag := weakETag(key, builtAt)
	b, _ := json.Marshal(resp)

	s.mediaMu.Lock()
	s.mediaRespJSON = b
	s.mediaKey = key
	s.mediaBuiltAt = builtAt
	s.mediaETag = etag
	s.mediaBuilding = false
	s.mediaCond.Broadcast()
	s.mediaMu.Unlock()

	if db.DB == nil {
		go s.saveMediaCacheToDisk(key, builtAt, etag, resp)
	}
	go debug.FreeOSMemory()
}

func mediaCacheKey(shares []config.Share, blacklist config.BlacklistConfig) string {
	var b strings.Builder
	b.WriteString(sharesCacheKey(shares))

	exts := normRuleList(blacklist.Extensions)
	files := normRuleList(blacklist.Filenames)
	folders := normRuleList(blacklist.Folders)

	b.WriteString("blExt=")
	b.WriteString(strings.Join(exts, ","))
	b.WriteByte('\n')
	b.WriteString("blFile=")
	b.WriteString(strings.Join(files, ","))
	b.WriteByte('\n')
	b.WriteString("blFolder=")
	b.WriteString(strings.Join(folders, ","))
	b.WriteByte('\n')
	b.WriteString("blSize=")
	b.WriteString(strings.TrimSpace(strings.ToLower(blacklist.SizeRule)))
	b.WriteByte('\n')

	return b.String()
}

func normRuleList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		out = append(out, strings.ToLower(v))
	}
	sort.Strings(out)
	return out
}

type mediaCacheOnDisk struct {
	Key     string              `json:"key"`
	BuiltAt int64               `json:"builtAt"`
	ETag    string              `json:"etag"`
	Resp    types.MediaResponse `json:"resp"`
}

func (s *Server) LoadMediaCacheFromDisk(key string) bool {
	if db.DB != nil {
		return false
	}
	s.mediaMu.Lock()
	already := s.mediaKey == key && !s.mediaBuiltAt.IsZero()
	need := s.mediaKey != key || s.mediaBuiltAt.IsZero()
	s.mediaMu.Unlock()
	if already || !need {
		return already
	}

	b, err := os.ReadFile(s.mediaCachePath)
	if err != nil || len(b) == 0 {
		return false
	}
	var v mediaCacheOnDisk
	if err := json.Unmarshal(b, &v); err != nil {
		return false
	}
	if v.Key != key || v.BuiltAt <= 0 {
		return false
	}

	s.mediaMu.Lock()
	s.mediaKey = v.Key
	s.mediaBuiltAt = time.Unix(0, v.BuiltAt)
	s.mediaETag = v.ETag
	s.mediaRespJSON, _ = json.Marshal(v.Resp)
	s.mediaMu.Unlock()
	return true
}

func (s *Server) saveMediaCacheToDisk(key string, builtAt time.Time, etag string, resp types.MediaResponse) {
	v := mediaCacheOnDisk{
		Key:     key,
		BuiltAt: builtAt.UnixNano(),
		ETag:    etag,
		Resp:    resp,
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	tmp := s.mediaCachePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.mediaCachePath)
}

func sharesCacheKey(shares []config.Share) string {
	s := append([]config.Share(nil), shares...)
	for i := range s {
		s[i].Path = util.NormalizePath(s[i].Path)
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
	return `W/` + util.U64Base36(h.Sum64()) + ``
}
