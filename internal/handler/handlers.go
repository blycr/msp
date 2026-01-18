package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"msp/internal/config"
	"msp/internal/db"
	"msp/internal/media"
	"msp/internal/server"
	"msp/internal/service"
	"msp/internal/types"
	"msp/internal/util"
)

type Handler struct {
	s             *server.Server
	configService *service.ConfigService
}

func New(s *server.Server) *Handler {
	return &Handler{
		s:             s,
		configService: service.NewConfigService(s),
	}
}

func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		view := h.configService.GetConfigView()
		writeJSON(w, http.StatusOK, view)
	case http.MethodPost:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, types.ConfigResponse{Error: &types.ApiError{Message: "JSON 解析失败"}})
			return
		}

		newCfg, err := h.configService.UpdateConfig(cfg)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, types.ConfigResponse{Error: &types.ApiError{Message: "写入配置失败"}})
			return
		}
		writeJSON(w, http.StatusOK, types.ConfigResponse{Config: newCfg})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req types.SharesOpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, types.SharesOpResponse{Error: &types.ApiError{Message: "JSON 解析失败"}})
		return
	}

	op := strings.ToLower(strings.TrimSpace(req.Op))
	p := util.NormalizePath(req.Path)
	label := strings.TrimSpace(req.Label)
	if label == "" && p != "" {
		label = filepath.Base(p)
	}

	var newCfg config.Config

	switch op {
	case "add":
		if p == "" || !util.IsExistingDir(p) {
			writeJSON(w, http.StatusBadRequest, types.SharesOpResponse{Error: &types.ApiError{Message: "目录不存在或不可访问"}})
			return
		}
	case "remove":
		if p == "" {
			writeJSON(w, http.StatusBadRequest, types.SharesOpResponse{Error: &types.ApiError{Message: "缺少 Path"}})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, types.SharesOpResponse{Error: &types.ApiError{Message: "不支持的 op（add/remove）"}})
		return
	}

	err := h.s.UpdateConfig(func(cfg *config.Config) {
		switch op {
		case "add":
			cfg.Shares = append(cfg.Shares, config.Share{Label: label, Path: p})
			cfg.Shares = util.NormalizeShares(cfg.Shares)
			cfg.Shares = util.DedupeShares(cfg.Shares)
		case "remove":
			out := make([]config.Share, 0, len(cfg.Shares))
			for _, sh := range cfg.Shares {
				if !util.SamePath(sh.Path, p) {
					out = append(out, sh)
				}
			}
			cfg.Shares = out
		}
		newCfg = *cfg
	})

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, types.SharesOpResponse{Error: &types.ApiError{Message: "写入配置失败"}})
		return
	}

	h.s.InvalidateMediaCache()
	writeJSON(w, http.StatusOK, types.SharesOpResponse{Config: newCfg})
}

func (h *Handler) HandleIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lanIPs": util.GetLanIPv4s(),
	})
}

func (h *Handler) HandlePrefs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		prefs, err := db.GetAllPrefs(r.Context())
		if err != nil {
			log.Printf("Error in GetAllPrefs: %v", err)
			writeJSON(w, http.StatusInternalServerError, types.PrefsResponse{Error: &types.ApiError{Message: "读取偏好失败"}})
			return
		}
		writeJSON(w, http.StatusOK, types.PrefsResponse{Prefs: prefs})
	case http.MethodPost:
		var req types.PrefsUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, types.PrefsResponse{Error: &types.ApiError{Message: "JSON 解析失败"}})
			return
		}
		if len(req.Prefs) == 0 {
			writeJSON(w, http.StatusBadRequest, types.PrefsResponse{Error: &types.ApiError{Message: "缺少 prefs"}})
			return
		}
		if err := db.SetPrefs(r.Context(), req.Prefs); err != nil {
			log.Printf("Error in SetPrefs: %v", err)
			writeJSON(w, http.StatusInternalServerError, types.PrefsResponse{Error: &types.ApiError{Message: "写入偏好失败"}})
			return
		}
		writeJSON(w, http.StatusOK, types.PrefsResponse{Prefs: req.Prefs})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleProgress(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		t, err := db.GetProgress(r.Context(), id)
		if err != nil {
			log.Printf("Error in GetProgress: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "读取进度失败"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"time": t})
	case http.MethodPost:
		var req struct {
			ID   string  `json:"id"`
			Time float64 `json:"time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "JSON 解析失败"})
			return
		}
		if req.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "缺少 id"})
			return
		}
		if err := db.SetProgress(r.Context(), req.ID, req.Time); err != nil {
			log.Printf("Error in SetProgress: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "保存进度失败"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req types.LogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Msg != "" {
		h.s.Log(req.Level, req.Msg)
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandlePIN validates PIN authentication
func (h *Handler) HandlePIN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cfg := h.s.Config()
	if !cfg.Security.PINEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   true,
			"enabled": false,
		})
		return
	}

	var req struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"valid": false,
			"error": "Invalid request",
		})
		return
	}

	valid := req.PIN == cfg.Security.PIN
	if valid {
		// Set cookie for future requests
		http.SetCookie(w, &http.Cookie{
			Name:     "msp_pin",
			Value:    req.PIN,
			Path:     "/",
			MaxAge:   86400 * 7, // 7 days
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":   valid,
		"enabled": true,
	})
}

func (h *Handler) HandleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cfg := h.s.Config()
	shares := append([]config.Share(nil), cfg.Shares...)
	blacklist := cfg.Blacklist

	refresh := r.URL.Query().Get("refresh") == "1"
	resp, etag := h.s.GetOrBuildMediaCache(r.Context(), shares, blacklist, refresh)

	resp.VideosTotal = len(resp.Videos)
	resp.AudiosTotal = len(resp.Audios)
	resp.ImagesTotal = len(resp.Images)
	resp.OthersTotal = len(resp.Others)

	limit := 0
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	if limit > 0 {
		if len(resp.Videos) > limit {
			resp.Videos = resp.Videos[:limit]
		}
		if len(resp.Audios) > limit {
			resp.Audios = resp.Audios[:limit]
		}
		if len(resp.Images) > limit {
			resp.Images = resp.Images[:limit]
		}
		if len(resp.Others) > limit {
			resp.Others = resp.Others[:limit]
		}
		resp.Limited = true
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if etag != "" {
		w.Header().Set("ETag", etag)
		if !refresh && strings.TrimSpace(r.Header.Get("If-None-Match")) == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	target, err := util.DecodeID(id)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	target = util.NormalizePath(target)

	cfg := h.s.Config()
	shares := append([]config.Share(nil), cfg.Shares...)

	if !util.IsAllowedFile(target, shares) {
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}

	f, err := os.Open(target)
	if err != nil {
		http.Error(w, "open failed", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

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

	// --- 智能转码决策逻辑 ---
	isAudio := media.ClassifyExt(ext) == "audio"
	isVideo := media.ClassifyExt(ext) == "video"

	// 判断是否应该触发转码逻辑 (FFmpeg)
	shouldTranscode := false

	// 策略修改：只有前端明确请求转码（?transcode=1）且配置允许时，才进行转码。
	// 默认情况下，所有文件（包括 MKV, AVI, FLAC 等）都尝试原生直接播放。
	// 前端如果播放失败（解码错误），会自动重试并带上 transcode=1 参数。

	if r.URL.Query().Get("transcode") == "1" {
		// 检查配置是否允许转码（作为安全开关）
		allowed := false
		if isVideo && cfg.Playback.Video.Transcode != nil && *cfg.Playback.Video.Transcode {
			allowed = true
		} else if isAudio && cfg.Playback.Audio.Transcode != nil && *cfg.Playback.Audio.Transcode {
			allowed = true
		}

		if allowed {
			shouldTranscode = true
		} else {
			// 如果请求了转码但配置不允许，直接返回 403，
			// 这样前端能明确知道“尝试转码失败”，而不是得到一个原生流再次报错。
			http.Error(w, "Transcoding is disabled in configuration", http.StatusForbidden)
			return
		}
	}

	canTranscode := media.CheckFFmpeg()

	if shouldTranscode && canTranscode {
		start, _ := strconv.ParseFloat(r.URL.Query().Get("start"), 64)
		opts := media.TranscodeOptions{
			Format:  r.URL.Query().Get("format"),
			Bitrate: r.URL.Query().Get("bitrate"),
			Offset:  start,
		}

		// 自动决定输出格式
		if isAudio && opts.Format == "" {
			opts.Format = "mp3"
		}

		stream, err := media.TranscodeStream(r.Context(), target, opts)
		if err == nil {
			defer func() { _ = stream.Close() }()
			if isAudio {
				w.Header().Set("Content-Type", "audio/mpeg")
			} else {
				w.Header().Set("Content-Type", "video/mp4")
			}
			w.Header().Set("X-MSP-Transcode", "1")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Del("Content-Length")
			_, _ = io.Copy(w, stream)
			return
		}
		// 如果启动转码失败，则回退到下面的原生播放
		log.Printf("[WARN] Transcode failed for %s, falling back to direct play: %v", target, err)
	}

	// --- 原生直接播放逻辑 (Direct Play) ---
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", st.Name()))

	http.ServeContent(w, r, st.Name(), time.Time{}, f)
}

func (h *Handler) HandleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, types.ProbeResponse{Error: &types.ApiError{Message: "missing id"}})
		return
	}

	target, err := util.DecodeID(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, types.ProbeResponse{Error: &types.ApiError{Message: "bad id"}})
		return
	}
	target = util.NormalizePath(target)

	cfg := h.s.Config()
	shares := append([]config.Share(nil), cfg.Shares...)

	if !util.IsAllowedFile(target, shares) {
		writeJSON(w, http.StatusForbidden, types.ProbeResponse{Error: &types.ApiError{Message: "not allowed"}})
		return
	}

	ext := strings.ToLower(filepath.Ext(target))
	video, audio := media.SniffContainerCodecs(target, ext)
	var subs []types.Subtitle
	if media.ClassifyExt(ext) == "video" {
		subs = media.FindSidecarSubtitles(target)
	}
	writeJSON(w, http.StatusOK, types.ProbeResponse{
		Container: strings.TrimPrefix(ext, "."),
		Video:     video,
		Audio:     audio,
		Subtitles: subs,
	})
}

func (h *Handler) HandleSubtitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	target, err := util.DecodeID(id)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	target = util.NormalizePath(target)

	cfg := h.s.Config()
	shares := append([]config.Share(nil), cfg.Shares...)

	if !util.IsAllowedFile(target, shares) {
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}

	f, err := os.Open(target)
	if err != nil {
		http.Error(w, "open failed", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

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
		out := media.SrtToVtt(b)
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Cache-Control", "private, max-age=0")
		http.ServeContent(w, r, strings.TrimSuffix(st.Name(), ext)+".vtt", st.ModTime(), bytes.NewReader(out))
		return
	default:
		http.Error(w, "unsupported subtitle format", http.StatusBadRequest)
		return
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}
