package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// FFmpeg/FFprobe 路径发现
var (
	ffmpegPath  string
	ffprobePath string
	pathOnce    sync.Once
)

// ResetPathsForTest resets the discovered paths for testing.
func ResetPathsForTest() {
	pathOnce = sync.Once{}
	ffmpegPath = ""
	ffprobePath = ""
}

// resolveFFmpegPaths discovers ffmpeg and ffprobe paths once.
func resolveFFmpegPaths() {
	pathOnce.Do(func() {
		ffmpegPath = findExecutable("ffmpeg")
		if ffmpegPath != "" {
			dir := filepath.Dir(ffmpegPath)
			candidate := filepath.Join(dir, exeName("ffprobe"))
			if _, err := os.Stat(candidate); err == nil {
				ffprobePath = candidate
			} else {
				ffprobePath = findExecutable("ffprobe")
			}
		} else {
			ffprobePath = findExecutable("ffprobe")
		}
	})
}

// FFmpegPath returns the discovered ffmpeg path, or empty string if not found.
func FFmpegPath() string {
	resolveFFmpegPaths()
	return ffmpegPath
}

// FFprobePath returns the discovered ffprobe path, or empty string if not found.
func FFprobePath() string {
	resolveFFmpegPaths()
	return ffprobePath
}

// FFmpegAvailable returns true if ffmpeg was found.
func FFmpegAvailable() bool {
	return FFmpegPath() != ""
}

func findExecutable(name string) string {
	exe := exeName(name)

	// 1. 环境变量（仅 ffmpeg）
	if name == "ffmpeg" {
		if env := os.Getenv("MSP_FFMPEG_PATH"); env != "" {
			if p, err := exec.LookPath(env); err == nil {
				return p
			}
			if _, err := os.Stat(env); err == nil {
				return env
			}
		}
	}

	// 2-5. 程序目录和工作目录
	for _, c := range localCandidatePaths(exe) {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// 6. 平台特定路径
	for _, c := range platformCandidatePaths(exe) {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// 7. 系统 PATH
	if p, err := exec.LookPath(exe); err == nil {
		return p
	}

	return ""
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func localCandidatePaths(exe string) []string {
	var paths []string

	if exePath, err := os.Executable(); err == nil {
		dir := filepath.Dir(exePath)
		paths = append(paths, filepath.Join(dir, exe))
		paths = append(paths, filepath.Join(dir, "bin", exe))
	}

	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, exe))
		paths = append(paths, filepath.Join(cwd, "bin", exe))
	}

	return paths
}

func platformCandidatePaths(exe string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(`C:\FFmpeg\bin`, exe),
			filepath.Join(`C:\Program Files\FFmpeg\bin`, exe),
		}
	case "darwin":
		return []string{
			filepath.Join("/opt/homebrew/bin", exe),
			filepath.Join("/usr/local/bin", exe),
		}
	default:
		return []string{
			filepath.Join("/usr/local/bin", exe),
			filepath.Join("/usr/bin", exe),
		}
	}
}

func CheckFFmpeg() bool {
	resolveFFmpegPaths()
	if ffmpegPath == "" {
		log.Printf("[WARN] FFmpeg not found (searched: MSP_FFMPEG_PATH, executable dir, ./bin, platform paths, PATH)")
		return false
	}
	log.Printf("[INFO] FFmpeg found: %s", ffmpegPath)
	return true
}

func CheckFFprobe() bool {
	resolveFFmpegPaths()
	return ffprobePath != ""
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
	probePath := FFprobePath()
	if probePath == "" {
		return CodecInfo{}, fmt.Errorf("ffprobe not found")
	}

	args := []string{
		"-v", "error",
		"-select_streams", "v:0,a:0",
		"-show_entries", "stream=codec_name,codec_type",
		"-of", "json",
		inputPath,
	}

	//nolint:gosec // Safe subprocess args
	cmd := exec.CommandContext(ctx, probePath, args...)
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
