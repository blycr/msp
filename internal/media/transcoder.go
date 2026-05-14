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

// TranscodeOptions defines transcode parameters.
type TranscodeOptions struct {
	Bitrate string  // Target bitrate, e.g. "2M"
	Format  string  // Target format, e.g. "mp4"
	Offset  float64 // Start offset (seconds)
}

// Allowed formats whitelist.
var allowedFormats = map[string]bool{
	"mp4":  true,
	"mp3":  true,
	"aac":  true,
	"webm": true,
	"ogg":  true,
}

// Validate validates transcode options.
func (opts *TranscodeOptions) Validate() error {
	if opts.Format == "" {
		opts.Format = "mp4"
	}
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if !allowedFormats[opts.Format] {
		return fmt.Errorf("unsupported format: %s", opts.Format)
	}

	if opts.Bitrate != "" {
		bitrate := strings.ToLower(strings.TrimSpace(opts.Bitrate))
		validBitrate := true
		for _, c := range bitrate {
			if (c < '0' || c > '9') && c != 'k' && c != 'm' {
				validBitrate = false
				break
			}
		}
		if !validBitrate {
			return fmt.Errorf("invalid bitrate format: %s", opts.Bitrate)
		}
		opts.Bitrate = bitrate
	}

	if opts.Offset < 0 {
		opts.Offset = 0
	}

	return nil
}

// limitReleaser wraps io.ReadCloser to release semaphore on Close.
type limitReleaser struct {
	io.ReadCloser
	limit chan struct{}
	once  sync.Once
}

func (l *limitReleaser) Close() error {
	l.once.Do(func() {
		<-l.limit
	})
	return l.ReadCloser.Close()
}

// TranscodeStream performs intelligent transcoding output.
func (mp *MediaProcessor) TranscodeStream(ctx context.Context, inputPath string, opts TranscodeOptions) (io.ReadCloser, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("input path not accessible: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("input path is a directory, not a file")
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input path is not a regular file")
	}

	ffmpegBin := mp.FFmpegPath()
	if ffmpegBin == "" {
		return nil, fmt.Errorf("FFmpeg not found")
	}

	// Acquire semaphore
	select {
	case mp.transcode.limit <- struct{}{}:
	default:
		return nil, fmt.Errorf("server busy: max transcode limit reached")
	}

	success := false
	defer func() {
		if !success {
			<-mp.transcode.limit
		}
	}()

	// 1. Try to get codec info
	codec, err := mp.GetCodecInfo(ctx, inputPath)
	if err != nil {
		log.Printf("[WARN] GetCodecInfo error for %s: %v", inputPath, err)
	}

	args := []string{"-hide_banner", "-loglevel", "error"}

	// 2. Smart argument selection
	if opts.Format == "mp3" || opts.Format == "aac" {
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
		if codec.VideoCodec == "h264" {
			if opts.Offset > 0 {
				args = append(args, "-ss", fmt.Sprintf("%f", opts.Offset))
			}
			args = append(args, "-i", inputPath)
			args = append(args, "-vcodec", "copy")
		} else {
			initArgs, codecArgs := mp.BuildVideoArgs(opts.Bitrate)
			args = append(args, initArgs...)
			if opts.Offset > 0 {
				args = append(args, "-ss", fmt.Sprintf("%f", opts.Offset))
			}
			args = append(args, "-i", inputPath)
			args = append(args, codecArgs...)
		}

		if codec.AudioCodec == "aac" || codec.AudioCodec == "mp3" {
			args = append(args, "-acodec", "copy")
		} else {
			args = append(args, "-acodec", "aac")
		}

		args = append(args, "-movflags", "frag_keyframe+empty_moov+default_base_moof+faststart")

		if opts.Offset > 0 {
			args = append(args, "-copyts")
		}
	}

	args = append(args, "-f", opts.Format, "-map_metadata", "-1", "pipe:1")

	//nolint:gosec // Safe subprocess args
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)

	// Track process for graceful shutdown
	mp.transcode.mu.Lock()
	mp.transcode.active[cmd] = struct{}{}
	mp.transcode.mu.Unlock()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		mp.removeProcess(cmd)
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		mp.removeProcess(cmd)
		return nil, fmt.Errorf("ffmpeg start error: %w (stderr: %s)", err, stderr.String())
	}

	go func() {
		_ = cmd.Wait()
		mp.removeProcess(cmd)
	}()

	success = true
	return &limitReleaser{ReadCloser: stdout, limit: mp.transcode.limit}, nil
}

// KillAllTranscodeProcesses kills all active ffmpeg processes for graceful shutdown.
func (mp *MediaProcessor) KillAllTranscodeProcesses() {
	mp.transcode.mu.Lock()
	defer mp.transcode.mu.Unlock()

	for cmd := range mp.transcode.active {
		if cmd.Process != nil {
			log.Printf("[INFO] Killing ffmpeg process %d", cmd.Process.Pid)
			_ = cmd.Process.Kill()
		}
	}
}

func (mp *MediaProcessor) removeProcess(cmd *exec.Cmd) {
	mp.transcode.mu.Lock()
	delete(mp.transcode.active, cmd)
	mp.transcode.mu.Unlock()
}
