package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type CodecInfo struct {
	VideoCodec string
	AudioCodec string
	Subtitles  []SubtitleTrack
}

// SubtitleTrack describes one embedded text-subtitle stream in the source file.
type SubtitleTrack struct {
	Index     int
	CodecName string
	Language  string
	Title     string
}

// isTextSubtitle reports whether the codec produces text subtitles that can be
// converted to WebVTT. Image-based subtitles (PGS/DVD/DVB) are excluded because
// they cannot be re-muxed to WebVTT without OCR.
func isTextSubtitle(codec string) bool {
	switch strings.ToLower(codec) {
	case "subrip", "ass", "ssa", "webvtt", "mov_text":
		return true
	}
	return false
}

// probeCacheEntry stores cached ffprobe results and expiration time.
type probeCacheEntry struct {
	info   CodecInfo
	mtime  int64
	expire time.Time
}

// SetProbeCacheTTL sets the ffprobe cache TTL (default 5 minutes).
func (mp *MediaProcessor) SetProbeCacheTTL(ttl time.Duration) {
	mp.probeTTL.Store(int64(ttl))
}

// ClearProbeCache clears all ffprobe caches.
func (mp *MediaProcessor) ClearProbeCache() {
	mp.probeCache = sync.Map{}
}

// resolveFFmpegPaths discovers ffmpeg and ffprobe paths once.
func (mp *MediaProcessor) resolveFFmpegPaths() {
	mp.probePaths.once.Do(func() {
		mp.probePaths.ffmpeg = findExecutable("ffmpeg")
		if mp.probePaths.ffmpeg != "" {
			dir := filepath.Dir(mp.probePaths.ffmpeg)
			candidate := filepath.Join(dir, exeName("ffprobe"))
			if _, err := os.Stat(candidate); err == nil {
				mp.probePaths.ffprobe = candidate
			} else {
				mp.probePaths.ffprobe = findExecutable("ffprobe")
			}
		} else {
			mp.probePaths.ffprobe = findExecutable("ffprobe")
		}
	})
}

// FFmpegPath returns the discovered ffmpeg path, or empty string if not found.
func (mp *MediaProcessor) FFmpegPath() string {
	mp.resolveFFmpegPaths()
	return mp.probePaths.ffmpeg
}

// FFprobePath returns the discovered ffprobe path, or empty string if not found.
func (mp *MediaProcessor) FFprobePath() string {
	mp.resolveFFmpegPaths()
	return mp.probePaths.ffprobe
}

// FFmpegAvailable returns true if ffmpeg was found.
func (mp *MediaProcessor) FFmpegAvailable() bool {
	if mp == nil {
		return false
	}
	return mp.FFmpegPath() != ""
}

func findExecutable(name string) string {
	exe := exeName(name)

	// 1. Environment variable (ffmpeg only)
	if name == "ffmpeg" {
		if env := os.Getenv("MSP_FFMPEG_PATH"); env != "" {
			if p, err := exec.LookPath(env); err == nil {
				return p
			}
			//nolint:gosec // env 来自用户配置的环境变量 MSP_FFMPEG_PATH
			if _, err := os.Stat(env); err == nil {
				return env
			}
		}
	}

	// 2-5. Program directory and working directory
	for _, c := range localCandidatePaths(exe) {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// 6. Platform-specific paths
	for _, c := range platformCandidatePaths(exe) {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// 7. System PATH
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
			filepath.Join("C:", "FFmpeg", "bin", exe),
			filepath.Join("C:", "Program Files", "FFmpeg", "bin", exe),
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

func (mp *MediaProcessor) CheckFFmpeg() bool {
	if mp == nil {
		return false
	}
	mp.resolveFFmpegPaths()
	if mp.probePaths.ffmpeg == "" {
		slog.Warn("FFmpeg not found (searched: MSP_FFMPEG_PATH, executable dir, ./bin, platform paths, PATH)")
		return false
	}
	version := mp.FFmpegVersion()
	if version == "" {
		slog.Info("FFmpeg found", "path", mp.probePaths.ffmpeg)
		return true
	}
	slog.Info("FFmpeg found", "path", mp.probePaths.ffmpeg, "version", version)
	if major, minor, ok := parseFFmpegVersionLine(version); ok && (major < 2 || (major == 2 && minor < 6)) {
		slog.Warn("FFmpeg version is older than 2.6; video/audio re-encode may fail", "version", version)
	}
	return true
}

// FFmpegVersion returns the first line of `ffmpeg -version` output, or "" if ffmpeg
// is unavailable or the query failed. The query runs at most once per process.
func (mp *MediaProcessor) FFmpegVersion() string {
	if mp == nil {
		return ""
	}
	mp.probePaths.verOnce.Do(func() {
		path := mp.FFmpegPath()
		if path == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		//nolint:gosec // path 来自处理器已发现的可信路径
		out, err := exec.CommandContext(ctx, path, "-hide_banner", "-version").CombinedOutput()
		if err != nil {
			slog.Warn("failed to query ffmpeg version", "path", path, "err", err)
			return
		}

		line, _, _ := strings.Cut(string(out), "\n")
		mp.probePaths.version = strings.TrimSpace(line)
	})
	return mp.probePaths.version
}

// parseFFmpegVersionLine extracts major/minor from the first line of
// `ffmpeg -version` output, e.g. `ffmpeg version 7.1.2 Copyright ...`. Returns ok=false
// for snapshot builds (`N-123456-gabc`, `git-...`) which are treated as unknown-newer.
func parseFFmpegVersionLine(line string) (major, minor int, ok bool) {
	const prefix = "ffmpeg version "
	if !strings.HasPrefix(line, prefix) {
		return 0, 0, false
	}
	token := line[len(prefix):]
	if i := strings.IndexByte(token, ' '); i >= 0 {
		token = token[:i]
	}
	if token == "" || token[0] < '0' || token[0] > '9' {
		return 0, 0, false
	}
	if i := strings.IndexByte(token, '-'); i >= 0 {
		token = token[:i]
	}
	parts := strings.SplitN(token, ".", 3)
	major = atoiLeadingDigits(parts[0])
	if len(parts) > 1 {
		minor = atoiLeadingDigits(parts[1])
	}
	return major, minor, true
}

// atoiLeadingDigits parses the leading digit run of s; returns 0 if none.
func atoiLeadingDigits(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (mp *MediaProcessor) CheckFFprobe() bool {
	if mp == nil {
		return false
	}
	mp.resolveFFmpegPaths()
	return mp.probePaths.ffprobe != ""
}

// getCacheKey generates a cache key (file path + modification time).
func getCacheKey(path string, mtime int64) string {
	return fmt.Sprintf("%s|%d", path, mtime)
}

// getFileMtime gets the file modification time.
func getFileMtime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

func (mp *MediaProcessor) GetCodecInfo(ctx context.Context, inputPath string) (CodecInfo, error) {
	mtime := getFileMtime(inputPath)
	cacheKey := getCacheKey(inputPath, mtime)

	if cached, ok := mp.probeCache.Load(cacheKey); ok {
		if entry, valid := cached.(probeCacheEntry); valid && time.Now().Before(entry.expire) {
			return entry.info, nil
		}
		mp.probeCache.Delete(cacheKey)
	}

	info, err := mp.probeCodecInfo(ctx, inputPath)
	if err != nil {
		return info, err
	}

	entry := probeCacheEntry{
		info:   info,
		mtime:  mtime,
		expire: time.Now().Add(time.Duration(mp.probeTTL.Load())),
	}
	mp.probeCache.Store(cacheKey, entry)

	return info, nil
}

// probeCodecInfo executes the ffprobe command to get encoding information.
func (mp *MediaProcessor) probeCodecInfo(ctx context.Context, inputPath string) (CodecInfo, error) {
	probePath := mp.FFprobePath()
	if probePath == "" {
		return CodecInfo{}, fmt.Errorf("ffprobe not found")
	}

	// 不要用 -select_streams：它只接受单个 stream specifier，逗号并列
	// （v:0,a:0,s）会让 avformat_match_stream_specifier 返回 EINVAL，
	// ffprobe 的 CHECK_END 直接 goto end，进程以非零码退出，整次探测失败。
	// 这里改为一次性输出全部 stream，由 parseProbeJSON 自行筛选。
	//
	// 另：language/title 属于 stream 的 tags，必须用 stream_tags= 前缀取；
	// 写成 stream=language,title 会被静默忽略（不会报错，但也不会有输出）。
	// disposition 用来识别 attached_pic（封面图），否则带封面的音频文件
	// 第一个 video 流会是 mjpeg，被误判成"需要转码的视频"。
	args := []string{
		"-v", "error",
		"-show_entries", "stream=index,codec_name,codec_type:stream_tags=language,title:stream_disposition=attached_pic",
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

// probeStream represents a single stream in ffprobe JSON output.
//
// language / title 在 ffprobe 的 JSON 里是嵌套在 streams[].tags 下的，
// attached_pic 在 streams[].disposition 下，都不是顶层字段。
type probeStream struct {
	Index     int    `json:"index"`
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
	Tags      struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

// probeResult represents the overall ffprobe JSON output structure.
type probeResult struct {
	Streams []probeStream `json:"streams"`
}

// parseProbeJSON extracts encoding and embedded text-subtitle information
// from ffprobe JSON output.
func parseProbeJSON(data []byte) (CodecInfo, error) {
	var result probeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return CodecInfo{}, err
	}

	var info CodecInfo
	for _, s := range result.Streams {
		switch s.CodecType {
		case "video":
			// 跳过封面图（attached_pic）。带封面的音频（.m4a/.mp3）里
			// 第一个 video 流往往是 mjpeg 封面，若计为视频编码，
			// decidePlaybackMode 会把纯音频判成"需要转码"。
			if s.Disposition.AttachedPic != 0 {
				continue
			}
			if info.VideoCodec == "" {
				info.VideoCodec = s.CodecName
			}
		case "audio":
			if info.AudioCodec == "" {
				info.AudioCodec = s.CodecName
			}
		case "subtitle":
			if !isTextSubtitle(s.CodecName) {
				continue // 图像字幕（PGS/DVD/DVB）无法转 webvtt，排除
			}
			info.Subtitles = append(info.Subtitles, SubtitleTrack{
				Index:     s.Index,
				CodecName: s.CodecName,
				Language:  s.Tags.Language,
				Title:     s.Tags.Title,
			})
		}
	}
	return info, nil
}
