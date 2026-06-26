package handler

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"msp/internal/constants"
	"msp/internal/service"
	"msp/internal/util"
)

// thumbSema bounds concurrent ffmpeg thumbnail generations. Acquired with a
// timeout (see thumbnailQueueTimeout) so burst requests queue instead of being
// rejected outright — a first-load burst of many <img> tags would otherwise
// race past a non-blocking semaphore and leave broken images.
var thumbSema = make(chan struct{}, 4) // 最多同时生成 4 张缩略图

// thumbnailQueueTimeout is how long a request waits for a generation slot
// before giving up. Long enough to absorb a burst, short enough to fail fast.
const thumbnailQueueTimeout = 8 * time.Second

// thumbnailSeekTime is the default seek offset for frame extraction.
const thumbnailSeekTime = "5"

// noStoreHeader prevents the browser from caching error responses so that
// frontend retry logic can re-request a thumbnail that previously failed.
func noStoreHeader(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

// HandleThumbnail serves video thumbnails.
// GET /api/thumbnail?id=xxx
func (h *Handler) HandleThumbnail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, constants.ErrMsgMissingID)
		return
	}

	filePath, err := h.idCodec.DecodeID(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	filePath = util.NormalizePath(filePath)

	//nolint:gosec
	if _, err := os.Stat(filePath); err != nil {
		noStoreHeader(w)
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	// 缩略图缓存目录
	exeDir := util.MustExeDir()
	thumbDir := filepath.Join(exeDir, "thumbs")
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(filePath)))
	thumbPath := filepath.Join(thumbDir, hash+".jpg")

	//nolint:gosec
	if info, err := os.Stat(thumbPath); err == nil && info.Size() > 0 {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, thumbPath)
		return
	}

	// 检查 ffmpeg 可用性
	if !h.processor.FFmpegAvailable() {
		noStoreHeader(w)
		writeError(w, http.StatusServiceUnavailable, "ffmpeg not available")
		return
	}

	// 限制并发：排队等待可用槽位，超时则返回 503（no-store，前端可重试）
	select {
	case thumbSema <- struct{}{}:
		defer func() { <-thumbSema }()
	case <-time.After(thumbnailQueueTimeout):
		noStoreHeader(w)
		writeError(w, http.StatusServiceUnavailable, "thumbnail generation busy")
		return
	case <-r.Context().Done():
		return
	}

	// 拿到槽位后再检查缓存：排队期间可能已被其它请求生成好
	//nolint:gosec
	if info, err := os.Stat(thumbPath); err == nil && info.Size() > 0 {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, thumbPath)
		return
	}

	if err := os.MkdirAll(thumbDir, 0750); err != nil {
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("thumbnail: mkdir failed: %v", err))
		noStoreHeader(w)
		writeError(w, http.StatusInternalServerError, "failed to create cache dir")
		return
	}

	ffmpegPath := h.processor.FFmpegPath()
	// 先尝试在 5 秒处截取；短视频（<5s）会失败，回退到首帧（-ss 0）。
	//nolint:gosec // ffmpegPath 来自处理器已发现的可信路径；filePath 经 DecodeID + os.Stat 验证；thumbPath 为本地缓存目录内的派生路径。
	output, ok := generateThumbnail(r, ffmpegPath, filePath, thumbPath, thumbnailSeekTime)
	if !ok {
		// 删除可能产生的空/损坏文件，避免下次命中"缓存存在但无效"分支
		_ = os.Remove(thumbPath)
		h.logger.Log(service.LogLevelInfo, fmt.Sprintf("thumbnail: retry from first frame for %s", filePath))
		var output2 []byte
		output2, ok = generateThumbnail(r, ffmpegPath, filePath, thumbPath, "0")
		if !ok {
			_ = os.Remove(thumbPath)
			h.logger.Log(service.LogLevelWarning, fmt.Sprintf("thumbnail: ffmpeg failed for %s:\n%s\n%s", filePath, string(output), string(output2)))
			noStoreHeader(w)
			writeError(w, http.StatusNotFound, "thumbnail generation failed")
			return
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, thumbPath)
}

// generateThumbnail runs ffmpeg to extract a single frame at seekTime into
// thumbPath. Returns the combined ffmpeg output and true on success.
//
//nolint:gosec // args derived from validated inputs
func generateThumbnail(r *http.Request, ffmpegPath, filePath, thumbPath, seekTime string) ([]byte, bool) {
	cmd := exec.CommandContext(r.Context(), ffmpegPath,
		"-ss", seekTime,
		"-i", filePath,
		"-vframes", "1",
		"-vf", "scale=320:-1",
		"-q:v", "8",
		"-y",
		thumbPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, false
	}
	return output, true
}
