package handler

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"msp/internal/config"
	"msp/internal/constants"
	"msp/internal/domain"
	"msp/internal/media"
	"msp/internal/scanner"
	"msp/internal/service"
	"msp/internal/util"
)

func (h *Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	target, f, st, err := h.resolveMediaTarget(w, r)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(st.Name()))
	ct := determineContentType(ext)
	cfg := h.config.Config()

	shouldTranscode, err := h.checkTranscodePolicy(r, cfg, ext)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	if shouldTranscode && h.processor.CheckFFmpeg() {
		// HLS 转码（视频 seek/Range 支持）：?hls=1 创建会话并返回播放列表 URL；
		// 失败时回退渐进式转码。
		if r.URL.Query().Get("hls") == "1" && h.tryServeHLS(w, r, target) {
			return
		}
		if h.tryServeTranscode(w, r, target, ext) {
			return
		}
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("Transcode failed for %s, falling back to direct play", target))
	}

	h.serveDirect(w, r, f, st, ct)
}

func (h *Handler) resolveMediaTarget(w http.ResponseWriter, r *http.Request) (string, *os.File, os.FileInfo, error) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return "", nil, nil, fmt.Errorf("missing id")
	}

	target, err := h.idCodec.DecodeID(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return "", nil, nil, err
	}
	//nolint:gosec // Validated via idCodec.DecodeID and IsAllowedFile below
	target = util.NormalizePath(target)

	cfg := h.config.Config()
	shares := append([]domain.Share(nil), cfg.Shares...)

	if !util.IsAllowedFile(target, shares) {
		writeError(w, http.StatusForbidden, "not allowed")
		return "", nil, nil, fmt.Errorf("not allowed")
	}

	//nolint:gosec // Path is validated above
	f, err := os.Open(target)
	if err != nil {
		writeError(w, http.StatusNotFound, "open failed")
		return "", nil, nil, err
	}

	// TOCTOU defense: re-resolve symlinks after open
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		_ = f.Close()
		writeError(w, http.StatusNotFound, "resolve failed")
		return "", nil, nil, fmt.Errorf("resolve failed")
	}
	resolvedTarget = util.NormalizePath(resolvedTarget)
	if !util.IsAllowedFile(resolvedTarget, shares) {
		_ = f.Close()
		writeError(w, http.StatusForbidden, "not allowed")
		return "", nil, nil, fmt.Errorf("not allowed")
	}

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		_ = f.Close()
		writeError(w, http.StatusNotFound, "not found")
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
	".wmv":  "video/x-ms-wmv",
	".mov":  "video/quicktime",
	".ts":   "video/mp2t",
	".vtt":  "text/vtt; charset=utf-8",
	".srt":  "text/plain; charset=utf-8",
	".lrc":  "text/plain; charset=utf-8",
}

// mediaExts lists extensions that are safe to serve inline.
// Non-media files are forced to attachment to prevent stored XSS.
var mediaExts = map[string]bool{
	".mp4": true, ".m4v": true, ".mkv": true, ".webm": true,
	".avi": true, ".wmv": true, ".mov": true, ".ts": true,
	".mp3": true, ".aac": true, ".ogg": true, ".flac": true,
	".wav": true, ".m4a": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".svg": true, ".bmp": true,
	".vtt": true, ".srt": true, ".lrc": true,
}

func isMediaExt(ext string) bool {
	return mediaExts[strings.ToLower(ext)]
}

func (h *Handler) checkTranscodePolicy(r *http.Request, cfg config.Config, ext string) (bool, error) {
	if r.URL.Query().Get("transcode") != "1" {
		return false, nil
	}

	isAudio := scanner.ClassifyExt(ext) == "audio"
	isVideo := scanner.ClassifyExt(ext) == "video"

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

// transcodeCopyBufPool provides reusable 256KB buffers for streaming
// transcoded output, reducing per-copy allocations and syscall count
// compared to io.Copy's default 32KB buffer.
var transcodeCopyBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 256*1024)
		return &b
	},
}

func (h *Handler) tryServeTranscode(w http.ResponseWriter, r *http.Request, target string, ext string) bool {
	isAudio := scanner.ClassifyExt(ext) == "audio"
	start, _ := strconv.ParseFloat(r.URL.Query().Get("start"), 64)
	opts := media.TranscodeOptions{
		Format:  r.URL.Query().Get("format"),
		Bitrate: r.URL.Query().Get("bitrate"),
		Offset:  start,
	}

	if isAudio && opts.Format == "" {
		opts.Format = "mp3"
	}

	stream, err := h.processor.TranscodeStream(r.Context(), target, opts)
	if err != nil {
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("Transcode stream error: %v", err))
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
	bufPtr := transcodeCopyBufPool.Get().(*[]byte)
	defer transcodeCopyBufPool.Put(bufPtr)
	if _, err := io.CopyBuffer(w, stream, *bufPtr); err != nil {
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("io.CopyBuffer transcode stream error: %v", err))
	}
	return true
}

// tryServeHLS starts an HLS transcode session and responds with the playlist
// URL. The session's segments are served via /api/hls/<sessionID>/<file>.
func (h *Handler) tryServeHLS(w http.ResponseWriter, r *http.Request, target string) bool {
	if h.processor == nil {
		return false
	}
	session, err := h.processor.StartHLSStream(target, media.TranscodeOptions{Format: "mp4"})
	if err != nil {
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("HLS stream error: %v", err))
		return false
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"m3u8": "/api/hls/" + session.ID + "/index.m3u8",
	})
	return true
}

func (h *Handler) serveDirect(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo, ct string) {
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Accept-Ranges", "bytes")

	if st.Size() > 10*1024*1024 {
		w.Header().Set("Cache-Control", "private, max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}

	disposition := "inline"
	if !isMediaExt(strings.ToLower(filepath.Ext(st.Name()))) {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, st.Name()))
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

func (h *Handler) HandleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, domain.ProbeResponse{Error: &domain.ApiError{Message: constants.ErrMsgMissingID}})
		return
	}

	target, err := h.idCodec.DecodeID(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, domain.ProbeResponse{Error: &domain.ApiError{Message: constants.ErrMsgBadID}})
		return
	}
	//nolint:gosec // Validated via idCodec.DecodeID
	target = util.NormalizePath(target)

	cfg := h.config.Config()
	shares := append([]domain.Share(nil), cfg.Shares...)

	if !util.IsAllowedFile(target, shares) {
		writeJSON(w, http.StatusForbidden, domain.ProbeResponse{Error: &domain.ApiError{Message: constants.ErrMsgNotAllowed}})
		return
	}

	ext := strings.ToLower(filepath.Ext(target))

	// 播放决策的主数据源是 ffprobe（精确、带 5 分钟缓存）；ffprobe 不可用或
	// 失败时回退字节嗅探。嗅探仅支持 MKV/MP4/MOV/M4V，其余容器返回空编码。
	var rawVideo, rawAudio string
	var info *media.CodecInfo
	if h.processor != nil {
		if ci, err := h.processor.GetCodecInfo(r.Context(), target); err == nil {
			info = &ci
			rawVideo, rawAudio = ci.VideoCodec, ci.AudioCodec
		}
	}
	if info == nil {
		rawVideo, rawAudio = scanner.SniffContainerCodecs(target, ext)
	}
	video := media.DisplayCodecName(rawVideo)
	audio := media.DisplayCodecName(rawAudio)

	var subs []domain.Subtitle
	if scanner.ClassifyExt(ext) == "video" {
		subs = scanner.FindSidecarSubtitles(target, h.idCodec)
		// 内嵌文本字幕（ffprobe 成功时可用）：与侧车合并，zh 优先排序，
		// 第一个轨道标记为默认（与 scanner 侧车字幕行为一致）。
		if info != nil && len(info.Subtitles) > 0 {
			subs = append(subs, embeddedSubtitles(id, info.Subtitles)...)
			sortSubtitleList(subs)
			subs[0].Default = true
		}
	}

	kind := scanner.ClassifyExt(ext)
	transcodeEnabled := false
	if kind == "video" && cfg.Playback.Video.Transcode != nil && *cfg.Playback.Video.Transcode {
		transcodeEnabled = true
	} else if kind == "audio" && cfg.Playback.Audio.Transcode != nil && *cfg.Playback.Audio.Transcode {
		transcodeEnabled = true
	}

	var playback *domain.PlaybackStrategy
	if transcodeEnabled {
		mode := decidePlaybackMode(rawVideo, rawAudio, h.processor.FFmpegAvailable())
		playback = &domain.PlaybackStrategy{Mode: mode}
	}

	writeJSON(w, http.StatusOK, domain.ProbeResponse{
		Container: strings.TrimPrefix(ext, "."),
		Video:     video,
		Audio:     audio,
		Subtitles: subs,
		Playback:  playback,
	})
}

// decidePlaybackMode determines the optimal playback strategy based on actual codecs.
// Returns "direct" or "transcode".
func decidePlaybackMode(videoCodec, audioCodec string, ffmpegAvailable bool) string {
	if !ffmpegAvailable {
		return "direct"
	}

	// 视频编码判断
	if videoCodec != "" {
		vc := strings.ToLower(videoCodec)
		switch {
		case strings.Contains(vc, "264") || strings.Contains(vc, "avc"):
			// H.264: browser native support
		case strings.Contains(vc, "265") || strings.Contains(vc, "hevc"):
			return "transcode"
		case strings.Contains(vc, "av1"):
			return "transcode"
		case strings.Contains(vc, "vc1") || strings.Contains(vc, "wmv3"):
			return "transcode"
		default:
			return "transcode" // unknown video codec: conservative
		}
	}

	// 音频编码判断（解决"有画无声"）
	if audioCodec != "" {
		ac := strings.ToLower(audioCodec)
		switch {
		case strings.Contains(ac, "aac"):
			// browser native support (includes "AAC", "AAC/MP4A")
		case strings.Contains(ac, "mp3") || ac == "mp3":
			// browser native support
		case strings.Contains(ac, "opus"):
			// browser native support
		case strings.Contains(ac, "vorbis"):
			// browser native support
		case strings.Contains(ac, "flac"):
			// Chrome supports, Firefox partial — conservative direct
		case ac == "pcm" || ac == "lpcm" || ac == "wav":
			// uncompressed: browser supports
		case strings.Contains(ac, "ac-3") || strings.Contains(ac, "ac3"):
			return "transcode" // AC-3/E-AC-3: Chrome silently skips
		case strings.Contains(ac, "dts") || strings.Contains(ac, "dca"):
			return "transcode"
		case strings.Contains(ac, "truehd"):
			return "transcode"
		default:
			return "transcode" // unknown audio codec: conservative
		}
	}

	return "direct"
}

// embeddedSubtitles converts ffprobe subtitle-track info into domain.Subtitle
// entries whose Src points at the extraction endpoint. mediaID is the encoded
// media-file ID used by /api/subtitle.
func embeddedSubtitles(mediaID string, tracks []media.SubtitleTrack) []domain.Subtitle {
	out := make([]domain.Subtitle, 0, len(tracks))
	for _, tr := range tracks {
		lang := tr.Language
		label := tr.Title
		if label == "" {
			label = scanner.SubtitleLabel(lang)
		}
		if label == "" {
			label = fmt.Sprintf("Track %d", tr.Index)
		}
		out = append(out, domain.Subtitle{
			Label: label,
			Lang:  lang,
			Src:   fmt.Sprintf("/api/subtitle?id=%s&track=%d", mediaID, tr.Index),
		})
	}
	return out
}

// sortSubtitleList orders subtitles zh-first then by label, mirroring
// scanner.sortSubtitles so the merged list has a stable default track.
func sortSubtitleList(out []domain.Subtitle) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lang == "zh" && out[j].Lang != "zh" {
			return true
		}
		if out[i].Lang != "zh" && out[j].Lang == "zh" {
			return false
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
}
