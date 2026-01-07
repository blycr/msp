package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"msp/internal/config"
	"msp/internal/db"
	"msp/internal/media"
	"msp/internal/types"
	"msp/internal/util"
)

type Server struct {
	mu      sync.RWMutex
	cfg     config.Config
	cfgPath string

	mediaCachePath string

	mediaMu       sync.Mutex
	mediaCond     *sync.Cond
	mediaKey      string
	mediaBuiltAt  time.Time
	mediaTTL      time.Duration
	mediaResp     types.MediaResponse
	mediaETag     string
	mediaBuilding bool

	seenIPs sync.Map
}

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

	var cfg config.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}

	changed := config.ApplyDefaults(&cfg)

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	if changed {
		return s.saveConfigLocked()
	}
	return nil
}

func (s *Server) saveConfigLocked() error {
	// Assumes s.mu is locked (read or write) by caller if reading s.cfg
	// But we need to marshal it.
	// If caller holds Lock, we are fine.

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

func (s *Server) SetupLogger() {
	s.mu.Lock()
	if s.cfg.LogFile == "" {
		s.cfg.LogFile = filepath.Join(util.MustExeDir(), "logs", "msp.log")
	}
	logFile := s.cfg.LogFile
	s.mu.Unlock()

	_ = os.MkdirAll(filepath.Dir(logFile), 0755)
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	log.SetOutput(f)
}

func (s *Server) LogRequest(r *http.Request, status int, start time.Time) {
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
	s.mediaResp = types.MediaResponse{}
	s.mediaMu.Unlock()
	_ = os.Remove(s.mediaCachePath)
}

func (s *Server) GetOrBuildMediaCache(shares []config.Share, blacklist config.BlacklistConfig, refresh bool) (types.MediaResponse, string) {
	key := mediaCacheKey(shares, blacklist)
	if !refresh {
		if resp, builtAt, ok, _ := media.LoadMediaFromDB(key, shares); ok && !builtAt.IsZero() {
			etag := weakETag(key, builtAt)
			s.mediaMu.Lock()
			s.mediaResp = resp
			s.mediaKey = key
			s.mediaBuiltAt = builtAt
			s.mediaETag = etag
			s.mediaMu.Unlock()
			return resp, etag
		}
		_ = s.LoadMediaCacheFromDisk(key)
	}

	for {
		s.mediaMu.Lock()
		has := s.mediaKey == key && !s.mediaBuiltAt.IsZero()
		if has && !refresh {
			if time.Since(s.mediaBuiltAt) < s.mediaTTL {
				resp := s.mediaResp
				etag := s.mediaETag
				s.mediaMu.Unlock()
				return resp, etag
			}
			resp := s.mediaResp
			etag := s.mediaETag
			if !s.mediaBuilding {
				s.mediaBuilding = true
				go s.rebuildMediaCache(key, shares, blacklist, s.cfg.MaxItems)
			}
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

		var resp types.MediaResponse
		builtAt := time.Now()
		if db.DB != nil {
			r, bt, err := media.ReindexAndLoadMedia(key, shares, blacklist, s.cfg.MaxItems)
			if err == nil && !bt.IsZero() {
				resp = r
				builtAt = bt
			} else {
				resp = media.BuildMediaResponse(shares, blacklist, s.cfg.MaxItems)
				builtAt = time.Now()
			}
		} else {
			resp = media.BuildMediaResponse(shares, blacklist, s.cfg.MaxItems)
		}
		etag := weakETag(key, builtAt)

		s.mediaMu.Lock()
		s.mediaResp = resp
		s.mediaKey = key
		s.mediaBuiltAt = builtAt
		s.mediaETag = etag
		s.mediaBuilding = false
		s.mediaCond.Broadcast()
		s.mediaMu.Unlock()
		if db.DB == nil {
			go s.saveMediaCacheToDisk(key, builtAt, etag, resp)
		}
		return resp, etag
	}
}

func (s *Server) rebuildMediaCache(key string, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) {
	var resp types.MediaResponse
	builtAt := time.Now()
	if db.DB != nil {
		r, bt, err := media.ReindexAndLoadMedia(key, shares, blacklist, maxItems)
		if err == nil && !bt.IsZero() {
			resp = r
			builtAt = bt
		} else {
			resp = media.BuildMediaResponse(shares, blacklist, maxItems)
			builtAt = time.Now()
		}
	} else {
		resp = media.BuildMediaResponse(shares, blacklist, maxItems)
	}
	etag := weakETag(key, builtAt)

	s.mediaMu.Lock()
	s.mediaResp = resp
	s.mediaKey = key
	s.mediaBuiltAt = builtAt
	s.mediaETag = etag
	s.mediaBuilding = false
	s.mediaCond.Broadcast()
	s.mediaMu.Unlock()
	if db.DB == nil {
		go s.saveMediaCacheToDisk(key, builtAt, etag, resp)
	}
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
	s.mediaResp = v.Resp
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
	return `W/"` + util.U64Base36(h.Sum64()) + `"`
}
