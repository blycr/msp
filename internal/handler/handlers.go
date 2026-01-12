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
	"msp/internal/types"
	"msp/internal/util"
)

type Handler struct {
	s *server.Server
}

func New(s *server.Server) *Handler {
	return &Handler{s: s}
}

func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ips := util.GetLanIPv4s()
		port := h.s.GetPort()
		urls := make([]string, 0, 2+len(ips))
		urls = append(urls, "http://127.0.0.1:"+util.Itoa(port)+"/")
		for _, ip := range ips {
			urls = append(urls, "http://"+ip+":"+util.Itoa(port)+"/")
		}

		cfg := h.s.Config()

		writeJSON(w, http.StatusOK, types.ConfigResponse{
			Config:  cfg,
			LanIPs:  ips,
			Urls:    urls,
			NowUnix: time.Now().Unix(),
		})
	case http.MethodPost:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, types.ConfigResponse{Error: &types.ApiError{Message: "JSON 解析失败"}})
			return
		}
		config.ApplyDefaults(&cfg)
		cfg.Shares = util.NormalizeShares(cfg.Shares)

		validShares := make([]config.Share, 0, len(cfg.Shares))
		for _, sh := range cfg.Shares {
			if sh.Path == "" {
				continue
			}
			p := util.NormalizePath(sh.Path)
			if ok := util.IsExistingDir(p); !ok {
				continue
			}
			sh.Path = p
			if sh.Label == "" {
				sh.Label = filepath.Base(p)
			}
			validShares = append(validShares, sh)
		}
		cfg.Shares = util.DedupeShares(validShares)

		err := h.s.UpdateConfig(func(c *config.Config) {
			*c = cfg
		})

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, types.ConfigResponse{Error: &types.ApiError{Message: "写入配置失败"}})
			return
		}
		h.s.InvalidateMediaCache()
		writeJSON(w, http.StatusOK, types.ConfigResponse{Config: cfg})
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
