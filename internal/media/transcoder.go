package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Global semaphore for limiting concurrent transcode sessions
// Limit to 2 concurrent sessions to prevent CPU starvation
var transcodeLimit = make(chan struct{}, 2)

// activeProcesses tracks running ffmpeg processes for graceful shutdown
var (
	activeProcesses   = make(map[*exec.Cmd]struct{})
	activeProcessesMu sync.Mutex
)

// SetTranscodeLimit replaces the global semaphore with one of the given size.
// Must be called before any transcode requests (typically at startup).
func SetTranscodeLimit(n int) {
	if n <= 0 {
		n = 2
	}
	transcodeLimit = make(chan struct{}, n)
	log.Printf("[INFO] Transcode concurrency limit set to %d", n)
}

// KillAllTranscodeProcesses kills all active ffmpeg processes for graceful shutdown
func KillAllTranscodeProcesses() {
	activeProcessesMu.Lock()
	defer activeProcessesMu.Unlock()

	for cmd := range activeProcesses {
		if cmd.Process != nil {
			log.Printf("[INFO] Killing ffmpeg process %d", cmd.Process.Pid)
			_ = cmd.Process.Kill()
		}
	}
}

// TranscodeOptions 定义转码参数
type TranscodeOptions struct {
	Bitrate string  // 目标码率，如 "2M"
	Format  string  // 目标格式，如 "mp4"
	Offset  float64 // 起始偏移量 (秒)
}

// 允许的转码格式白名单
var allowedFormats = map[string]bool{
	"mp4":  true,
	"mp3":  true,
	"aac":  true,
	"webm": true,
	"ogg":  true,
}

// 验证转码选项
func (opts *TranscodeOptions) Validate() error {
	// 验证格式
	if opts.Format == "" {
		opts.Format = "mp4"
	}
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if !allowedFormats[opts.Format] {
		return fmt.Errorf("不支持的格式: %s", opts.Format)
	}

	// 验证码率（如果提供）
	if opts.Bitrate != "" {
		// 只允许数字和特定后缀
		bitrate := strings.ToLower(strings.TrimSpace(opts.Bitrate))
		// 匹配格式如: 128k, 2M, 1000, 等
		validBitrate := true
		for _, c := range bitrate {
			if (c < '0' || c > '9') && c != 'k' && c != 'm' {
				validBitrate = false
				break
			}
		}
		if !validBitrate {
			return fmt.Errorf("无效的码率格式: %s", opts.Bitrate)
		}
		opts.Bitrate = bitrate
	}

	// 验证偏移量
	if opts.Offset < 0 {
		opts.Offset = 0
	}

	return nil
}

// limitReleaser wraps io.ReadCloser to release semaphore on Close
type limitReleaser struct {
	io.ReadCloser
	once sync.Once
}

func (l *limitReleaser) Close() error {
	l.once.Do(func() {
		<-transcodeLimit
	})
	return l.ReadCloser.Close()
}

// TranscodeStream 执行智能转码输出
func TranscodeStream(ctx context.Context, inputPath string, opts TranscodeOptions) (io.ReadCloser, error) {
	// 验证转码选项
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Validate input path exists and is a regular file
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("input path not accessible: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("input path is a directory, not a file")
	}

	// 验证是常规文件（不是符号链接、设备等）
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input path is not a regular file")
	}

	// Acquire semaphore
	select {
	case transcodeLimit <- struct{}{}:
		// Acquired
	default:
		return nil, fmt.Errorf("server busy: max transcode limit reached")
	}

	// Helper to release if we fail before returning
	success := false
	defer func() {
		if !success {
			<-transcodeLimit
		}
	}()

	// 1. 尝试获取编码信息
	codec, _ := GetCodecInfo(ctx, inputPath)

	args := []string{"-hide_banner", "-loglevel", "error"}

	// 2. 智能决定参数
	if opts.Format == "mp3" || opts.Format == "aac" {
		// 音频模式 — no hardware acceleration needed
		if opts.Offset > 0 {
			args = append(args, "-ss", fmt.Sprintf("%f", opts.Offset))
		}
		args = append(args, "-i", inputPath)

		if codec.AudioCodec == opts.Format {
			args = append(args, "-acodec", "copy")
		} else {
			args = append(args, "-acodec", "libmp3lame")
			if opts.Bitrate != "" {
				args = append(args, "-b:a", opts.Bitrate)
			}
		}
	} else {
		// 视频模式 (目标 MP4)
		// 视频流策略：如果是 h264 则 copy，否则转码
		if codec.VideoCodec == "h264" {
			// H.264 source → stream copy, no re-encode needed
			if opts.Offset > 0 {
				args = append(args, "-ss", fmt.Sprintf("%f", opts.Offset))
			}
			args = append(args, "-i", inputPath)
			args = append(args, "-vcodec", "copy")
		} else {
			// Needs re-encode: delegate encoder selection to BuildVideoArgs
			// which transparently handles hardware / software fallback.
			initArgs, codecArgs := BuildVideoArgs(opts.Bitrate)
			args = append(args, initArgs...)
			if opts.Offset > 0 {
				args = append(args, "-ss", fmt.Sprintf("%f", opts.Offset))
			}
			args = append(args, "-i", inputPath)
			args = append(args, codecArgs...)
		}

		// 音频流策略：如果是 aac/mp3 则 copy，否则转 aac
		if codec.AudioCodec == "aac" || codec.AudioCodec == "mp3" {
			args = append(args, "-acodec", "copy")
		} else {
			args = append(args, "-acodec", "aac")
		}

		// 优化 MP4 输出格式，添加 faststart 优化网络传输
		args = append(args, "-movflags", "frag_keyframe+empty_moov+default_base_moof+faststart")

		// 保留时间戳，以便前端进度条能正确显示位置
		if opts.Offset > 0 {
			args = append(args, "-copyts")
		}
	}

	args = append(args, "-f", opts.Format, "-map_metadata", "-1", "pipe:1")

	//nolint:gosec // Safe subprocess args
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Track process for graceful shutdown
	activeProcessesMu.Lock()
	activeProcesses[cmd] = struct{}{}
	activeProcessesMu.Unlock()

	// 捕获 stderr 用于调试
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		removeProcess(cmd)
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		removeProcess(cmd)
		return nil, fmt.Errorf("ffmpeg start error: %w (stderr: %s)", err, stderr.String())
	}

	go func() {
		_ = cmd.Wait()
		removeProcess(cmd)
	}()

	success = true
	return &limitReleaser{ReadCloser: stdout}, nil
}

func removeProcess(cmd *exec.Cmd) {
	activeProcessesMu.Lock()
	delete(activeProcesses, cmd)
	activeProcessesMu.Unlock()
}
