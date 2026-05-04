package handler

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"msp/internal/constants"
	"msp/internal/scanner"
)

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
	case ".ass", ".ssa":
		h.serveASS(w, r, f, st)
	default:
		writeError(w, http.StatusBadRequest, constants.ErrMsgUnsupportedFormat)
	}
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
