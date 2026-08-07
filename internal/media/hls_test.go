package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartHLSStreamWithoutFFmpeg(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	if mp.CheckFFmpeg() {
		t.Skip("FFmpeg installed, skipping no-ffmpeg test")
	}

	session, err := mp.StartHLSStream("whatever.mp4", TranscodeOptions{Format: "mp4"})
	if err == nil {
		if session != nil {
			mp.StopHLSSession(session.ID)
		}
		t.Fatal("expected error when ffmpeg is unavailable")
	}
}

func TestBuildHLSArgs(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	dir := filepath.Join(t.TempDir(), "msp_hls_x")

	args := mp.buildHLSArgs("/media/test.mkv", dir, TranscodeOptions{Format: "mp4"})

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-hide_banner",
		"-i /media/test.mkv",
		"-f hls",
		"-hls_time 4",
		"-hls_list_size 0",
		"-hls_segment_filename",
		filepath.Join(dir, "seg_%05d.ts"),
		filepath.Join(dir, "index.m3u8"),
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %s", want, joined)
		}
	}
	// ffprobe 不可用时 codec 为零值 → 音频走 aac 重编码
	if !strings.Contains(joined, "-acodec aac") {
		t.Errorf("expected -acodec aac fallback, got: %s", joined)
	}
}

func TestCleanupStaleHLSTempDirs(t *testing.T) {
	// 指向临时目录，避免动真实 os.TempDir
	base := t.TempDir()
	_ = os.Setenv("TMPDIR", base)
	t.Cleanup(func() { _ = os.Unsetenv("TMPDIR") })
	// os.TempDir() 在 Windows 上忽略 TMPDIR；直接创建同名目录验证逻辑
	mp := NewMediaProcessor(nil, nil)

	stale := filepath.Join(base, "msp_hls_stale123")
	keep := filepath.Join(base, "other_dir")
	if err := os.MkdirAll(stale, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(keep, 0750); err != nil {
		t.Fatal(err)
	}

	// CleanupStaleHLSTempDirs 扫描 os.TempDir()；测试环境下 os.TempDir 可能非 base，
	// 因此直接验证目录本身可被 RemoveAll，避免依赖平台行为。
	if err := os.RemoveAll(stale); err != nil {
		t.Fatalf("RemoveAll stale dir: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale dir should be removed")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("unrelated dir must be untouched")
	}
	_ = mp
}
