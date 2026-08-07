package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// hlsFileRe whitelists files servable from an HLS session directory. Segments
// are named seg_%05d.ts by ffmpeg; anything else is rejected to block path
// traversal / arbitrary file reads.
var hlsFileRe = regexp.MustCompile(`^(index\.m3u8|seg_\d{5}\.ts)$`)

// waitForFile polls until path exists on disk or the timeout elapses.
// The first HLS segment/playlist takes a few seconds to appear after ffmpeg
// starts, and the player may request it immediately.
func waitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil { //nolint:gosec // path 已由调用方正则 + Clean 校验
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// HandleHLS serves m3u8 playlists and TS segments for HLS transcode sessions:
//
//	GET /api/hls/<sessionID>/index.m3u8
//	GET /api/hls/<sessionID>/seg_00000.ts
//
// Sessions are created via GET /api/stream?id=...&transcode=1&hls=1.
func (h *Handler) HandleHLS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/hls/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusBadRequest, "bad hls path")
		return
	}
	sid, file := parts[0], parts[1]

	if h.processor == nil {
		writeError(w, http.StatusServiceUnavailable, "media processor unavailable")
		return
	}
	session := h.processor.HLSSession(sid)
	if session == nil {
		writeError(w, http.StatusNotFound, "hls session not found")
		return
	}
	session.Touch()

	if !hlsFileRe.MatchString(file) {
		writeError(w, http.StatusBadRequest, "invalid hls file")
		return
	}

	full := filepath.Join(session.Dir, file)
	if file == "index.m3u8" {
		if !waitForFile(full, 10*time.Second) {
			writeError(w, http.StatusNotFound, "hls playlist not ready")
			return
		}
	} else if _, err := os.Stat(full); err != nil { //nolint:gosec // file 经正则白名单，路径在会话目录内
		writeError(w, http.StatusNotFound, "hls segment not found")
		return
	}

	// 双重防护：正则已限制文件名，这里再确保 Clean 后仍在会话目录内
	clean := filepath.Clean(full)
	if clean != full || !strings.HasPrefix(clean, session.Dir+string(os.PathSeparator)) {
		writeError(w, http.StatusBadRequest, "invalid hls file")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	//nolint:gosec // full 已通过正则白名单 + Clean 前缀双重校验
	http.ServeFile(w, r, full)
}
