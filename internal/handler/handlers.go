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

	op, p, label := normalizeSharesOp(req)
	newCfg, err := h.applySharesOp(op, p, label)

	if err != nil {
		if strings.Contains(err.Error(), "exists") || strings.Contains(err.Error(), "missing") {
			writeJSON(w, http.StatusBadRequest, types.SharesOpResponse{Error: &types.ApiError{Message: err.Error()}})
		} else {
			writeJSON(w, http.StatusInternalServerError, types.SharesOpResponse{Error: &types.ApiError{Message: "写入配置失败"}})
		}
		return
	}

	h.s.InvalidateMediaCache()
	writeJSON(w, http.StatusOK, types.SharesOpResponse{Config: newCfg})
}

func normalizeSharesOp(req types.SharesOpRequest) (op string, path string, label string) {
	op = strings.ToLower(strings.TrimSpace(req.Op))
	path = util.NormalizePath(req.Path)
	label = strings.TrimSpace(req.Label)
	if label == "" && path != "" {
		label = filepath.Base(path)
	}
	return op, path, label
}

func (h *Handler) applySharesOp(op string, path string, label string) (config.Config, error) {
	switch op {
	case "add":
		return h.handleShareAdd(path, label)
	case "remove":
		return h.handleShareRemove(path)
	default:
		return config.Config{}, fmt.Errorf("不支持的 op（add/remove）")
	}
}

func (h *Handler) handleShareAdd(p, label string) (config.Config, error) {
	if p == "" || !util.IsExistingDir(p) {
		return config.Config{}, fmt.Errorf("目录不存在或不可访问")
	}

	var newCfg config.Config
	err := h.s.UpdateConfig(func(cfg *config.Config) {
		cfg.Shares = append(cfg.Shares, config.Share{Label: label, Path: p})
		cfg.Shares = util.NormalizeShares(cfg.Shares)
		cfg.Shares = util.DedupeShares(cfg.Shares)
		newCfg = *cfg
	})
	return newCfg, err
}

func (h *Handler) handleShareRemove(p string) (config.Config, error) {
	if p == "" {
		return config.Config{}, fmt.Errorf("缺少 Path")
	}

	var newCfg config.Config
	err := h.s.UpdateConfig(func(cfg *config.Config) {
		out := make([]config.Share, 0, len(cfg.Shares))
		for _, sh := range cfg.Shares {
			if !util.SamePath(sh.Path, p) {
				out = append(out, sh)
			}
		}
		cfg.Shares = out
		newCfg = *cfg
	})
	return newCfg, err
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

	limit := parseLimitParam(r)
	if applyLimit(&resp, limit) {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if writeNotModifiedIfMatch(w, r, etag, refresh) {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseLimitParam(r *http.Request) int {
	v := strings.TrimSpace(r.URL.Query().Get("limit"))
	if v == "" {
		return 0
	}
	limit, _ := strconv.Atoi(v)
	return limit
}

func applyLimit(resp *types.MediaResponse, limit int) bool {
	if limit <= 0 {
		return false
	}
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
	return true
}

func writeNotModifiedIfMatch(w http.ResponseWriter, r *http.Request, etag string, refresh bool) bool {
	if etag == "" {
		return false
	}
	w.Header().Set("ETag", etag)
	if refresh {
		return false
	}
	if strings.TrimSpace(r.Header.Get("If-None-Match")) == etag {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}

func (h *Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	target, f, st, err := h.resolveMediaTarget(w, r)
	if err != nil {
		// resolveMediaTarget handles the error response
		return
	}
	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(st.Name()))
	ct := determineContentType(ext)
	cfg := h.s.Config()

	// Check Transcoding Policy
	shouldTranscode, err := h.checkTranscodePolicy(r, cfg, ext)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if shouldTranscode && media.CheckFFmpeg() {
		if h.tryServeTranscode(w, r, target, ext) {
			return
		}
		log.Printf("[WARN] Transcode failed for %s, falling back to direct play", target)
	}

	// Direct Play
	h.serveDirect(w, r, f, st, ct)
}

func (h *Handler) resolveMediaTarget(w http.ResponseWriter, r *http.Request) (string, *os.File, os.FileInfo, error) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return "", nil, nil, fmt.Errorf("missing id")
	}

	target, err := util.DecodeID(id)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return "", nil, nil, err
	}
	//nolint:gosec // Validated via util.DecodeID and IsAllowedFile below
	target = util.NormalizePath(target)

	cfg := h.s.Config()
	shares := append([]config.Share(nil), cfg.Shares...)

	if !util.IsAllowedFile(target, shares) {
		http.Error(w, "not allowed", http.StatusForbidden)
		return "", nil, nil, fmt.Errorf("not allowed")
	}

	//nolint:gosec // Path is validated above
	f, err := os.Open(target)
	if err != nil {
		http.Error(w, "open failed", http.StatusNotFound)
		return "", nil, nil, err
	}

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		_ = f.Close()
		http.Error(w, "not found", http.StatusNotFound)
		return "", nil, nil, fmt.Errorf("not found")
	}

	return target, f, st, nil
}

func determineContentType(ext string) string {
	if ct, ok := contentTypeByExt[ext]; ok {
		return ct
	}
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}

var contentTypeByExt = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".mkv":  "video/x-matroska",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".ts":   "video/mp2t",
	".vtt":  "text/vtt; charset=utf-8",
	".srt":  "text/plain; charset=utf-8",
	".lrc":  "text/plain; charset=utf-8",
}

func (h *Handler) checkTranscodePolicy(r *http.Request, cfg config.Config, ext string) (bool, error) {
	if r.URL.Query().Get("transcode") != "1" {
		return false, nil
	}

	isAudio := media.ClassifyExt(ext) == "audio"
	isVideo := media.ClassifyExt(ext) == "video"

	allowed := false
	if isVideo && cfg.Playback.Video.Transcode != nil && *cfg.Playback.Video.Transcode {
		allowed = true
	} else if isAudio && cfg.Playback.Audio.Transcode != nil && *cfg.Playback.Audio.Transcode {
		allowed = true
	}

	if !allowed {
		return false, fmt.Errorf("transcoding is disabled in configuration")
	}
	return true, nil
}

func (h *Handler) tryServeTranscode(w http.ResponseWriter, r *http.Request, target string, ext string) bool {
	isAudio := media.ClassifyExt(ext) == "audio"
	start, _ := strconv.ParseFloat(r.URL.Query().Get("start"), 64)
	opts := media.TranscodeOptions{
		Format:  r.URL.Query().Get("format"),
		Bitrate: r.URL.Query().Get("bitrate"),
		Offset:  start,
	}

	if isAudio && opts.Format == "" {
		opts.Format = "mp3"
	}

	stream, err := media.TranscodeStream(r.Context(), target, opts)
	if err != nil {
		log.Printf("[WARN] Transcode stream error: %v", err)
		return false
	}
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
	return true
}

func (h *Handler) serveDirect(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo, ct string) {
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
	//nolint:gosec // Validated via util.DecodeID
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

	_, f, st, err := h.resolveMediaTarget(w, r)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(st.Name()))
	switch ext {
	case ".vtt":
		h.serveVTT(w, r, f, st)
	case ".srt":
		h.serveSRT(w, r, f, st)
	default:
		http.Error(w, "unsupported subtitle format", http.StatusBadRequest)
	}
}

func (h *Handler) serveVTT(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo) {
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

func (h *Handler) serveSRT(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo) {
	b, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	out := media.SrtToVtt(b)
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, strings.TrimSuffix(st.Name(), filepath.Ext(st.Name()))+".vtt", st.ModTime(), bytes.NewReader(out))
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
