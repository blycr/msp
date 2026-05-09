package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

type CodecInfo struct {
	VideoCodec string
	AudioCodec string
}

// probeCacheEntry 存储缓存的 ffprobe 结果和过期时间
type probeCacheEntry struct {
	info   CodecInfo
	mtime  int64
	expire time.Time
}

var (
	probeCache sync.Map
	cacheTTL   = 5 * time.Minute
)

// SetProbeCacheTTL 设置 ffprobe 缓存的 TTL（默认 5 分钟）
func SetProbeCacheTTL(ttl time.Duration) {
	cacheTTL = ttl
}

// ClearProbeCache 清除所有 ffprobe 缓存
func ClearProbeCache() {
	probeCache = sync.Map{}
}

func CheckFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("[WARN] FFmpeg not found in PATH")
	}
	return err == nil
}

func CheckFFprobe() bool {
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

// getCacheKey 生成缓存键（文件路径 + 修改时间）
func getCacheKey(path string, mtime int64) string {
	return fmt.Sprintf("%s|%d", path, mtime)
}

// getFileMtime 获取文件修改时间
func getFileMtime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

func GetCodecInfo(ctx context.Context, inputPath string) (CodecInfo, error) {
	// 获取文件修改时间用于缓存键
	mtime := getFileMtime(inputPath)
	cacheKey := getCacheKey(inputPath, mtime)

	// 检查缓存
	if cached, ok := probeCache.Load(cacheKey); ok {
		if entry, valid := cached.(probeCacheEntry); valid && time.Now().Before(entry.expire) {
			return entry.info, nil
		}
		// 缓存过期，删除
		probeCache.Delete(cacheKey)
	}

	// 执行 ffprobe 获取编码信息
	info, err := probeCodecInfo(ctx, inputPath)
	if err != nil {
		return info, err
	}

	// 存入缓存
	entry := probeCacheEntry{
		info:   info,
		mtime:  mtime,
		expire: time.Now().Add(cacheTTL),
	}
	probeCache.Store(cacheKey, entry)

	return info, nil
}

// probeCodecInfo 实际执行 ffprobe 命令（单次调用获取视频和音频编码）
func probeCodecInfo(ctx context.Context, inputPath string) (CodecInfo, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0,a:0",
		"-show_entries", "stream=codec_name,codec_type",
		"-of", "json",
		inputPath,
	}

	//nolint:gosec // Safe subprocess args
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return CodecInfo{}, err
	}

	return parseProbeJSON(out)
}

// probeStream 表示 ffprobe JSON 输出中的单个流
type probeStream struct {
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
}

// probeResult 表示 ffprobe JSON 输出的整体结构
type probeResult struct {
	Streams []probeStream `json:"streams"`
}

// parseProbeJSON 从 ffprobe 的 JSON 输出中提取编码信息
func parseProbeJSON(data []byte) (CodecInfo, error) {
	var result probeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return CodecInfo{}, err
	}

	var info CodecInfo
	for _, s := range result.Streams {
		switch s.CodecType {
		case "video":
			if info.VideoCodec == "" {
				info.VideoCodec = s.CodecName
			}
		case "audio":
			if info.AudioCodec == "" {
				info.AudioCodec = s.CodecName
			}
		}
	}
	return info, nil
}
