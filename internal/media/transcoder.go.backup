package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
)

// Global semaphore for limiting concurrent transcode sessions
// Limit to 2 concurrent sessions to prevent CPU starvation
var transcodeLimit = make(chan struct{}, 2)

// TranscodeOptions 定义转码参数
type TranscodeOptions struct {
	Bitrate string  // 目标码率，如 "2M"
	Format  string  // 目标格式，如 "mp4"
	Offset  float64 // 起始偏移量 (秒)
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

// CodecInfo 存储媒体编码信息
type CodecInfo struct {
	VideoCodec string
	AudioCodec string
}

// CheckFFmpeg 探测 ffmpeg 是否安装
func CheckFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("[WARN] FFmpeg not found in PATH")
	}
	return err == nil
}

// CheckFFprobe 探测 ffprobe 是否安装
func CheckFFprobe() bool {
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

// GetCodecInfo 使用 ffprobe 获取文件的编码信息
func GetCodecInfo(ctx context.Context, inputPath string) (CodecInfo, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	}

	var info CodecInfo

	// 获取视频编码
	//nolint:gosec // Safe subprocess args
	cmdV := exec.CommandContext(ctx, "ffprobe", args...)
	outV, err := cmdV.Output()
	if err == nil {
		info.VideoCodec = strings.TrimSpace(string(outV))
	}

	// 获取音频编码
	args[2] = "a:0"
	//nolint:gosec // Safe subprocess args
	cmdA := exec.CommandContext(ctx, "ffprobe", args...)
	outA, err := cmdA.Output()
	if err == nil {
		info.AudioCodec = strings.TrimSpace(string(outA))
	}

	return info, nil
}

// TranscodeStream 执行智能转码输出
func TranscodeStream(ctx context.Context, inputPath string, opts TranscodeOptions) (io.ReadCloser, error) {
	// Acquire semaphore
	select {
	case transcodeLimit <- struct{}{}:
		// Acquired
	default:
		return nil, fmt.Errorf("server busy: max transcode limit reached")
	}

	if opts.Format == "" {
		opts.Format = "mp4"
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
	if opts.Offset > 0 {
		args = append(args, "-ss", fmt.Sprintf("%f", opts.Offset))
	}
	args = append(args, "-i", inputPath)

	// 2. 智能决定参数
	if opts.Format == "mp3" || opts.Format == "aac" {
		// 音频模式
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
			args = append(args, "-vcodec", "copy")
		} else {
			args = append(args, "-vcodec", "libx264", "-pix_fmt", "yuv420p")
			if opts.Bitrate != "" {
				args = append(args, "-b:v", opts.Bitrate)
			}
		}

		// 音频流策略：如果是 aac/mp3 则 copy，否则转 aac
		if codec.AudioCodec == "aac" || codec.AudioCodec == "mp3" {
			args = append(args, "-acodec", "copy")
		} else {
			args = append(args, "-acodec", "aac")
		}

		args = append(args, "-movflags", "frag_keyframe+empty_moov+default_base_moof")

		// 保留时间戳，以便前端进度条能正确显示位置
		if opts.Offset > 0 {
			args = append(args, "-copyts")
		}
	}

	args = append(args, "-f", opts.Format, "-map_metadata", "-1", "pipe:1")

	//nolint:gosec // Safe subprocess args
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// 捕获 stderr 用于调试
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg start error: %w (stderr: %s)", err, stderr.String())
	}

	go func() {
		_ = cmd.Wait()
	}()

	success = true
	return &limitReleaser{ReadCloser: stdout}, nil
}
