package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HLS session defaults.
const (
	hlssessionTimeout     = 5 * time.Minute // 无访问自动清理
	hlssessionJanitorTick = 30 * time.Second
	hlssessionSegmentTime = "4" // 段时长（秒），决定 seek 精度
)

// HLSSession represents one live video-transcode-to-HLS session. The ffmpeg
// process writes segments into Dir; the m3u8 playlist and segments are served
// over /api/hls/<ID>/<file>. Sessions are reaped by the janitor once
// LastAccess falls behind the timeout.
type HLSSession struct {
	ID         string
	Dir        string
	Cmd        *exec.Cmd
	LastAccess atomic.Int64 // unix seconds

	cancel context.CancelFunc
}

// Touch refreshes the session's idle timestamp. Called by the serving handler.
func (s *HLSSession) Touch() {
	s.LastAccess.Store(time.Now().Unix())
}

// stop cancels the ffmpeg process and removes the session's temp directory.
// The transcode-limit slot is released by the cmd.Wait goroutine.
func (s *HLSSession) stop() {
	s.cancel()
	_ = os.RemoveAll(s.Dir)
}

type hlsManager struct {
	mu       sync.Mutex
	sessions map[string]*HLSSession
	started  sync.Once
}

// StartHLSStream spawns ffmpeg to transcode inputPath into an HLS playlist
// (4s segments, full-length list) under a fresh temp directory. The session
// is registered and served via /api/hls/<ID>/<file>. The returned session's
// ID is used to build the playlist URL.
func (mp *MediaProcessor) StartHLSStream(inputPath string, opts TranscodeOptions) (*HLSSession, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	info, err := os.Lstat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("input path not accessible: %w", err)
	}
	if info.IsDir() || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input path is not a regular file")
	}

	ffmpegBin := mp.FFmpegPath()
	if ffmpegBin == "" {
		return nil, fmt.Errorf("FFmpeg not found")
	}

	// Acquire a transcode slot (same semaphore as progressive transcoding).
	select {
	case mp.transcode.limit <- struct{}{}:
	default:
		return nil, fmt.Errorf("server busy: max transcode limit reached")
	}

	dir, err := os.MkdirTemp("", "msp_hls_")
	if err != nil {
		<-mp.transcode.limit
		return nil, fmt.Errorf("create hls temp dir: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	args := mp.buildHLSArgs(inputPath, dir, opts)
	//nolint:gosec // inputPath 已校验，args 由内部构建
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)

	// Track process for graceful shutdown (same map as progressive transcodes).
	mp.transcode.mu.Lock()
	mp.transcode.active[cmd] = struct{}{}
	mp.transcode.mu.Unlock()

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		mp.removeProcess(cmd)
		cancel()
		_ = os.RemoveAll(dir)
		<-mp.transcode.limit
		return nil, fmt.Errorf("ffmpeg hls start error: %w (stderr: %s)", err, stderr.String())
	}

	session := &HLSSession{
		ID:     randomSessionID(),
		Dir:    dir,
		Cmd:    cmd,
		cancel: cancel,
	}
	session.Touch()

	mp.hls.mu.Lock()
	mp.hls.sessions[session.ID] = session
	mp.hls.mu.Unlock()
	mp.ensureHLSJanitor()

	// Release the transcode slot and active-map entry when ffmpeg exits
	// (naturally, on cancel, or via KillAllTranscodeProcesses).
	go func() {
		_ = cmd.Wait()
		mp.removeProcess(cmd)
		<-mp.transcode.limit
	}()

	slog.Info("HLS session started", "id", session.ID, "input", inputPath, "dir", dir)
	return session, nil
}

// HLSSession returns the session with the given ID, or nil.
func (mp *MediaProcessor) HLSSession(id string) *HLSSession {
	mp.hls.mu.Lock()
	defer mp.hls.mu.Unlock()
	return mp.hls.sessions[id]
}

// StopHLSSession cancels and removes a session immediately.
func (mp *MediaProcessor) StopHLSSession(id string) {
	mp.hls.mu.Lock()
	s, ok := mp.hls.sessions[id]
	if ok {
		delete(mp.hls.sessions, id)
	}
	mp.hls.mu.Unlock()
	if ok {
		s.stop()
	}
}

// CleanupStaleHLSTempDirs removes leftover msp_hls_* temp dirs from a
// previous crash. Called once at startup.
func (mp *MediaProcessor) CleanupStaleHLSTempDirs() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "msp_hls_") {
			_ = os.RemoveAll(filepath.Join(os.TempDir(), e.Name()))
		}
	}
}

// ensureHLSJanitor starts the idle-reaper goroutine exactly once.
func (mp *MediaProcessor) ensureHLSJanitor() {
	mp.hls.started.Do(func() {
		go func() {
			ticker := time.NewTicker(hlssessionJanitorTick)
			defer ticker.Stop()
			for range ticker.C {
				mp.cleanupExpiredHLSSessions()
			}
		}()
	})
}

// cleanupExpiredHLSSessions stops sessions idle past the timeout.
func (mp *MediaProcessor) cleanupExpiredHLSSessions() {
	cutoff := time.Now().Add(-hlssessionTimeout).Unix()

	mp.hls.mu.Lock()
	var expired []*HLSSession
	for id, s := range mp.hls.sessions {
		if s.LastAccess.Load() < cutoff {
			expired = append(expired, s)
			delete(mp.hls.sessions, id)
		}
	}
	mp.hls.mu.Unlock()

	for _, s := range expired {
		slog.Info("HLS session expired, cleaning up", "id", s.ID)
		s.stop()
	}
}

// buildHLSArgs assembles the ffmpeg command line for HLS output.
// Codec decisions mirror TranscodeStream: stream-copy H.264/AAC, otherwise
// hardware/software re-encode.
func (mp *MediaProcessor) buildHLSArgs(inputPath, dir string, opts TranscodeOptions) []string {
	codec, err := mp.GetCodecInfo(context.Background(), inputPath)
	if err != nil {
		slog.Warn("GetCodecInfo error", "path", inputPath, "err", err)
	}

	args := []string{"-hide_banner", "-loglevel", "error"}

	if codec.VideoCodec == "h264" {
		args = append(args, "-i", inputPath, "-vcodec", "copy")
	} else {
		initArgs, codecArgs := mp.BuildVideoArgs(opts.Bitrate)
		args = append(args, initArgs...)
		args = append(args, "-i", inputPath)
		args = append(args, codecArgs...)
	}

	if codec.AudioCodec == "aac" || codec.AudioCodec == "mp3" {
		args = append(args, "-acodec", "copy")
	} else {
		args = append(args, "-acodec", "aac")
	}

	args = append(args,
		"-map_metadata", "-1",
		"-f", "hls",
		"-hls_time", hlssessionSegmentTime,
		"-hls_list_size", "0",
		"-hls_allow_cache", "1",
		"-hls_segment_filename", filepath.Join(dir, "seg_%05d.ts"),
		filepath.Join(dir, "index.m3u8"),
	)
	return args
}

func randomSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
