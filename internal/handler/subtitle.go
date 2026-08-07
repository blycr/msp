package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"msp/internal/constants"
	"msp/internal/scanner"
	"msp/internal/service"
)

func (h *Handler) HandleSubtitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	target, f, st, err := h.resolveMediaTarget(w, r)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	// 内嵌字幕提取：?track=N 指向媒体文件内的字幕轨道（文件路径已由
	// resolveMediaTarget 校验）。轨道号上限 63 与探测端一致。
	if trackStr := r.URL.Query().Get("track"); trackStr != "" {
		track, aerr := strconv.Atoi(trackStr)
		if aerr != nil || track < 0 || track > 63 {
			writeError(w, http.StatusBadRequest, "invalid track")
			return
		}
		h.serveEmbeddedSubtitle(w, r, target, track)
		return
	}

	ext := strings.ToLower(filepath.Ext(st.Name()))
	switch ext {
	case ".vtt":
		h.serveVTT(w, r, f, st)
	case ".srt":
		h.serveSRT(w, r, f, st)
	case ".ass", ".ssa":
		h.serveASS(w, r, f, st)
	default:
		writeError(w, http.StatusBadRequest, constants.ErrMsgUnsupportedFormat)
	}
}

// serveEmbeddedSubtitle extracts an embedded subtitle track (by stream index)
// from the media file and serves it as WebVTT. Only text tracks are expected
// here — image-based tracks are filtered out at probe time.
func (h *Handler) serveEmbeddedSubtitle(w http.ResponseWriter, r *http.Request, target string, track int) {
	if h.processor == nil || h.processor.FFmpegPath() == "" {
		writeError(w, http.StatusServiceUnavailable, "ffmpeg not available")
		return
	}
	ffmpegBin := h.processor.FFmpegPath()

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-i", target,
		"-map", fmt.Sprintf("0:s:%d", track),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"pipe:1",
	}
	//nolint:gosec // target 已通过 resolveMediaTarget 校验，track 为整数
	cmd := exec.CommandContext(r.Context(), ffmpegBin, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("embedded subtitle extraction failed (track %d): %v", track, err))
		writeError(w, http.StatusNotFound, "subtitle extraction failed")
		return
	}
	if int64(out.Len()) > maxSubtitleConvertSize {
		writeError(w, http.StatusRequestEntityTooLarge, "subtitle too large")
		return
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, "subtitle.vtt", time.Time{}, bytes.NewReader(out.Bytes()))
}

func (h *Handler) serveVTT(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo) {
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

func (h *Handler) serveSRT(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo) {
	if st.Size() > maxSubtitleConvertSize {
		writeError(w, http.StatusRequestEntityTooLarge, "subtitle too large")
		return
	}
	b, err := io.ReadAll(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, constants.ErrMsgReadFailed)
		return
	}
	out := scanner.SrtToVtt(b)
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, strings.TrimSuffix(st.Name(), filepath.Ext(st.Name()))+".vtt", st.ModTime(), bytes.NewReader(out))
}

func (h *Handler) serveASS(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo) {
	if st.Size() > maxSubtitleConvertSize {
		writeError(w, http.StatusRequestEntityTooLarge, "subtitle too large")
		return
	}
	b, err := io.ReadAll(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, constants.ErrMsgReadFailed)
		return
	}
	out := scanner.AssToVtt(b)
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, strings.TrimSuffix(st.Name(), filepath.Ext(st.Name()))+".vtt", st.ModTime(), bytes.NewReader(out))
}
