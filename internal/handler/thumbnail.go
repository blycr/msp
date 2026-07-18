package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"msp/internal/constants"
	"msp/internal/domain"
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

// thumbnailCacheControl 是缩略图成功响应的 Cache-Control。
// 缓存键是 sha256(文件路径)，不含 mtime：文件内容变化而路径不变时会命中旧缓存，
// 因此 max-age 不能取更大的量级，7 天是稳妥上限。
const thumbnailCacheControl = "public, max-age=604800"

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
		w.Header().Set("Cache-Control", thumbnailCacheControl)
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
		w.Header().Set("Cache-Control", thumbnailCacheControl)
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
	output, ok := generateThumbnail(r.Context(), ffmpegPath, filePath, thumbPath, thumbnailSeekTime)
	if !ok {
		// 删除可能产生的空/损坏文件，避免下次命中"缓存存在但无效"分支
		//nolint:gosec // G703: thumbPath is a local cache path derived from a SHA-256 hash
		_ = os.Remove(thumbPath)
		h.logger.Log(service.LogLevelInfo, fmt.Sprintf("thumbnail: retry from first frame for %s", filePath))
		var output2 []byte
		output2, ok = generateThumbnail(r.Context(), ffmpegPath, filePath, thumbPath, "0")
		if !ok {
			//nolint:gosec // G703: thumbPath is a local cache path derived from a SHA-256 hash
			_ = os.Remove(thumbPath)
			h.logger.Log(service.LogLevelWarning, fmt.Sprintf("thumbnail: ffmpeg failed for %s:\n%s\n%s", filePath, string(output), string(output2)))
			noStoreHeader(w)
			writeError(w, http.StatusNotFound, "thumbnail generation failed")
			return
		}
	}

	w.Header().Set("Cache-Control", thumbnailCacheControl)
	//nolint:gosec // G703: thumbPath is a local cache path derived from a SHA-256 hash
	http.ServeFile(w, r, thumbPath)
}

// generateThumbnail runs ffmpeg to extract a single frame at seekTime into
// thumbPath. Returns the combined ffmpeg output and true on success.
//
//nolint:gosec // args derived from validated inputs
func generateThumbnail(ctx context.Context, ffmpegPath, filePath, thumbPath, seekTime string) ([]byte, bool) {
	cmd := exec.CommandContext(ctx, ffmpegPath,
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

// ---- 扫描后的闲时预热 ----

// warmCacheBytes 是 OS 页缓存预热时对每个媒体文件读取的头/尾字节数。
const warmCacheBytes = 512 * 1024

// thumbWarmup 跟踪当前正在后台运行的预热任务，以便新扫描开始或服务关闭时取消。
var thumbWarmup struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// StartPostScanWarmup 由 MediaProcessor 的 post-scan hook 在扫描完成后调用，
// 取消上一轮预热并在后台启动新一轮：为还没有缓存文件的视频/图片预生成缩略图，
// 同时低并发地预热媒体文件的 OS 页缓存。items 为 nil 表示新扫描开始或服务
// 关闭，此时只取消上一轮预热。
func (h *Handler) StartPostScanWarmup(items []domain.MediaItem) {
	thumbWarmup.mu.Lock()
	if thumbWarmup.cancel != nil {
		thumbWarmup.cancel()
		thumbWarmup.cancel = nil
	}
	if len(items) == 0 {
		thumbWarmup.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	thumbWarmup.cancel = cancel
	thumbWarmup.mu.Unlock()

	go h.pregenerateThumbnails(ctx, items)
	go warmPageCache(ctx, items)
}

// pregenerateThumbnails 逐个为缺少缓存文件的视频/图片生成缩略图。复用与
// HandleThumbnail 相同的缓存路径、信号量与生成函数；最多占用 thumbSema 的
// 一个槽位，失败静默跳过，ctx 取消时立即停止。
func (h *Handler) pregenerateThumbnails(ctx context.Context, items []domain.MediaItem) {
	if h.processor == nil || !h.processor.FFmpegAvailable() {
		return
	}
	thumbDir := filepath.Join(util.MustExeDir(), "thumbs")
	ffmpegPath := h.processor.FFmpegPath()
	madeDir := false

	for _, item := range items {
		if item.Kind != "video" && item.Kind != "image" {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}

		filePath := util.NormalizePath(item.Path)
		if filePath == "" {
			continue
		}
		thumbPath := filepath.Join(thumbDir, fmt.Sprintf("%x", sha256.Sum256([]byte(filePath)))+".jpg")
		//nolint:gosec // thumbPath 是由 SHA-256 哈希派生的本地缓存路径
		if info, err := os.Stat(thumbPath); err == nil && info.Size() > 0 {
			continue
		}
		//nolint:gosec // filePath 来自本库扫描结果
		if _, err := os.Stat(filePath); err != nil {
			continue
		}

		if !madeDir {
			if err := os.MkdirAll(thumbDir, 0750); err != nil {
				return
			}
			madeDir = true
		}

		select {
		case thumbSema <- struct{}{}:
		case <-ctx.Done():
			return
		}
		// 拿到槽位后复查缓存：排队期间可能已被请求路径生成。
		//nolint:gosec // thumbPath 是由 SHA-256 哈希派生的本地缓存路径
		if info, err := os.Stat(thumbPath); err == nil && info.Size() > 0 {
			<-thumbSema
			continue
		}
		if _, ok := generateThumbnail(ctx, ffmpegPath, filePath, thumbPath, thumbnailSeekTime); !ok {
			//nolint:gosec // G703: thumbPath 是由 SHA-256 哈希派生的本地缓存路径
			_ = os.Remove(thumbPath)
			if _, ok := generateThumbnail(ctx, ffmpegPath, filePath, thumbPath, "0"); !ok {
				//nolint:gosec // G703: thumbPath 是由 SHA-256 哈希派生的本地缓存路径
				_ = os.Remove(thumbPath)
			}
		}
		<-thumbSema
	}
}

// warmPageCache 逐个读取媒体文件的头/尾各 warmCacheBytes 字节，让 OS page
// cache 变热；应用层不缓存任何数据。单 goroutine 顺序读取，失败静默跳过。
func warmPageCache(ctx context.Context, items []domain.MediaItem) {
	buf := make([]byte, warmCacheBytes)
	for _, item := range items {
		select {
		case <-ctx.Done():
			return
		default:
		}
		path := util.NormalizePath(item.Path)
		if path == "" {
			continue
		}
		warmOneFile(buf, path)
	}
}

// warmOneFile 读取单个文件的头/尾各 len(buf) 字节，数据读后即弃。
func warmOneFile(buf []byte, path string) {
	//nolint:gosec // path 来自本库扫描结果
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	// 头部；文件不足 len(buf) 时 ReadAt 返回 io.EOF，属正常。
	_, _ = f.ReadAt(buf, 0)
	if info, err := f.Stat(); err == nil && info.Size() > int64(len(buf)) {
		_, _ = f.ReadAt(buf, info.Size()-int64(len(buf))) // 尾部
	}
}
