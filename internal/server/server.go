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
	"msp/internal/media"
	"msp/internal/types"
	"msp/internal/util"
)

type Server struct {
	mu      sync.RWMutex
	cfg     config.Config
	cfgPath string

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
	s := &Server{cfgPath: cfgPath}
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
		s.cfg.LogFile = filepath.Join(util.MustExeDir(), "msp.log")
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
}

func (s *Server) GetOrBuildMediaCache(shares []config.Share, blacklist config.BlacklistConfig, refresh bool) (types.MediaResponse, string) {
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

		resp := media.BuildMediaResponse(shares, blacklist)
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

func sharesCacheKey(shares []config.Share) string {
	s := append([]config.Share(nil), shares...)
	for i := range s {
		s[i].Path = util.NormalizeWinPath(s[i].Path)
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
