package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"msp/internal/domain"
)

var (
	lastRefreshTime time.Time
	refreshMu       sync.Mutex
	refreshCooldown = 30 * time.Second
)

func (h *Handler) HandleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cfg := h.config.Config()
	shares := append([]domain.Share(nil), cfg.Shares...)
	blacklist := cfg.Blacklist

	refresh := r.URL.Query().Get("refresh") == "1"
	if refresh && getAccessLevelFromRequest(r) != AccessLocal {
		refreshMu.Lock()
		if time.Since(lastRefreshTime) < refreshCooldown {
			refreshMu.Unlock()
			writeError(w, http.StatusTooManyRequests, "refresh cooldown")
			return
		}
		lastRefreshTime = time.Now()
		refreshMu.Unlock()
	}
	resp, etag := h.media.GetOrBuildMediaCache(r.Context(), shares, blacklist, refresh)

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
	limit, err := strconv.Atoi(v)
	if err != nil || limit < 0 {
		return 0
	}
	return limit
}

func applyLimit(resp *domain.MediaResponse, limit int) bool {
	if limit <= 0 {
		return false
	}
	limited := false
	if len(resp.Videos) > limit {
		resp.Videos = resp.Videos[:limit]
		limited = true
	}
	if len(resp.Audios) > limit {
		resp.Audios = resp.Audios[:limit]
		limited = true
	}
	if len(resp.Images) > limit {
		resp.Images = resp.Images[:limit]
		limited = true
	}
	if len(resp.Others) > limit {
		resp.Others = resp.Others[:limit]
		limited = true
	}
	resp.Limited = limited
	return limited
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
