package handler

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"msp/internal/constants"
	"msp/internal/service"
	"msp/internal/util"
)

var thumbSema = make(chan struct{}, 2) // 最多同时生成 2 张缩略图

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

	// 检查文件是否存在
	if _, err := os.Stat(filePath); err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	// 缩略图缓存目录
	exeDir := util.MustExeDir()
	thumbDir := filepath.Join(exeDir, "thumbs")
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(filePath)))
	thumbPath := filepath.Join(thumbDir, hash+".jpg")

	// 检查缓存
	if info, err := os.Stat(thumbPath); err == nil && info.Size() > 0 {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, thumbPath)
		return
	}

	// 检查 ffmpeg 可用性
	if !h.processor.FFmpegAvailable() {
		writeError(w, http.StatusServiceUnavailable, "ffmpeg not available")
		return
	}

	// 限制并发
	select {
	case thumbSema <- struct{}{}:
		defer func() { <-thumbSema }()
	default:
		writeError(w, http.StatusTooManyRequests, "thumbnail generation busy")
		return
	}

	// 生成缩略图
	if err := os.MkdirAll(thumbDir, 0750); err != nil {
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("thumbnail: mkdir failed: %v", err))
		writeError(w, http.StatusInternalServerError, "failed to create cache dir")
		return
	}

	ffmpegPath := h.processor.FFmpegPath()
	// #nosec G204 — ffmpegPath 来自处理器已发现的可信路径；filePath 经 DecodeID + os.Stat 验证；thumbPath 为本地缓存目录内的派生路径。
	cmd := exec.CommandContext(r.Context(), ffmpegPath,
		"-ss", "5",
		"-i", filePath,
		"-vframes", "1",
		"-vf", "scale=320:-1",
		"-q:v", "8",
		"-y",
		thumbPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		h.logger.Log(service.LogLevelWarning, fmt.Sprintf("thumbnail: ffmpeg failed for %s: %v\n%s", filePath, err, string(output)))
		// 如果生成失败，直接返回 404，不报错，让前端隐藏
		writeError(w, http.StatusNotFound, "thumbnail generation failed")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, thumbPath)
}
