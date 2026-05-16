package media

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranscodeOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    TranscodeOptions
		wantErr bool
		after   TranscodeOptions
	}{
		{
			name:    "default format",
			opts:    TranscodeOptions{},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4", Bitrate: "", Offset: 0},
		},
		{
			name:    "valid format mp4",
			opts:    TranscodeOptions{Format: "mp4"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4"},
		},
		{
			name:    "valid format mp3",
			opts:    TranscodeOptions{Format: "mp3"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp3"},
		},
		{
			name:    "valid format aac",
			opts:    TranscodeOptions{Format: "AAC"},
			wantErr: false,
			after:   TranscodeOptions{Format: "aac"},
		},
		{
			name:    "invalid format",
			opts:    TranscodeOptions{Format: "exe"},
			wantErr: true,
		},
		{
			name:    "valid bitrate",
			opts:    TranscodeOptions{Format: "mp4", Bitrate: "2M"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4", Bitrate: "2m"},
		},
		{
			name:    "valid bitrate k",
			opts:    TranscodeOptions{Format: "mp3", Bitrate: "128k"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp3", Bitrate: "128k"},
		},
		{
			name:    "invalid bitrate format",
			opts:    TranscodeOptions{Format: "mp4", Bitrate: "2M;rm -rf /"},
			wantErr: true,
		},
		{
			name:    "negative offset",
			opts:    TranscodeOptions{Format: "mp4", Offset: -10},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4", Offset: 0},
		},
		{
			name:    "normal offset",
			opts:    TranscodeOptions{Format: "mp4", Offset: 30.5},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4", Offset: 30.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.after.Format != "" {
					assert.Equal(t, tt.after.Format, tt.opts.Format)
				}
				if tt.after.Bitrate != "" || tt.opts.Bitrate == "" {
					assert.Equal(t, tt.after.Bitrate, tt.opts.Bitrate)
				}
				assert.Equal(t, tt.after.Offset, tt.opts.Offset)
			}
		})
	}
}

func TestCheckFFmpeg(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	result := mp.CheckFFmpeg()
	_ = result
}

func TestCheckFFprobe(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	result := mp.CheckFFprobe()
	_ = result
}

func TestGetCodecInfo(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	if !mp.CheckFFprobe() {
		t.Skip("FFprobe not installed, skipping test")
	}

	tmpDir := t.TempDir()
	fakeMP4 := filepath.Join(tmpDir, "test.mp4")

	mp4Header := []byte{
		0x00, 0x00, 0x00, 0x18,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x00,
		'i', 's', 'o', 'm',
	}
	require.NoError(t, os.WriteFile(fakeMP4, mp4Header, 0600))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := mp.GetCodecInfo(ctx, fakeMP4)
	_ = err
	_ = info
}

func TestTranscodeStreamValidation(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	tmpDir := t.TempDir()

	t.Run("directory instead of file", func(t *testing.T) {
		opts := TranscodeOptions{Format: "mp4"}
		_, err := mp.TranscodeStream(context.Background(), tmpDir, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory")
	})

	t.Run("nonexistent file", func(t *testing.T) {
		opts := TranscodeOptions{Format: "mp4"}
		_, err := mp.TranscodeStream(context.Background(), "/nonexistent/file.mp4", opts)
		assert.Error(t, err)
	})

	t.Run("invalid format", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.mp4")
		require.NoError(t, os.WriteFile(testFile, []byte("fake"), 0600))

		opts := TranscodeOptions{Format: "invalid"}
		_, err := mp.TranscodeStream(context.Background(), testFile, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid options")
	})

	t.Run("non-regular file (symlink)", func(t *testing.T) {
		if os.PathSeparator == '\\' {
			t.Skip("skip Windows symlink test")
		}

		targetFile := filepath.Join(tmpDir, "target.mp4")
		linkFile := filepath.Join(tmpDir, "link.mp4")
		require.NoError(t, os.WriteFile(targetFile, []byte("fake"), 0600))
		require.NoError(t, os.Symlink(targetFile, linkFile))

		opts := TranscodeOptions{Format: "mp4"}
		_, err := mp.TranscodeStream(context.Background(), linkFile, opts)
		assert.Error(t, err)
	})
}

func TestTranscodeStreamConcurrency(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	if !mp.CheckFFmpeg() {
		t.Skip("FFmpeg not installed, skipping test")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")

	cmd := createTestVideo(testFile)
	if cmd != nil {
		err := cmd.Run()
		if err != nil {
			t.Skip("cannot create test video:", err)
		}
	}

	ctx := context.Background()
	opts := TranscodeOptions{Format: "mp4"}

	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			stream, err := mp.TranscodeStream(ctx, testFile, opts)
			if err == nil && stream != nil {
				_, _ = io.Copy(io.Discard, stream)
				_ = stream.Close()
			}
			done <- err
		}()
	}

	var busyCount int
	for i := 0; i < 5; i++ {
		err := <-done
		if err != nil && contains(err.Error(), "busy") {
			busyCount++
		}
	}

	assert.Greater(t, busyCount, 0, "some requests should fail due to concurrency limit")
}

func TestLimitReleaser(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	mp.transcode.limit = make(chan struct{}, 2)

	mp.transcode.limit <- struct{}{}
	assert.Equal(t, 1, len(mp.transcode.limit))

	pr, pw := io.Pipe()
	lr := &limitReleaser{ReadCloser: pr, limit: mp.transcode.limit}

	err := lr.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(mp.transcode.limit))

	err = lr.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(mp.transcode.limit))

	_ = pw.Close()
}

func TestSetTranscodeLimit(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)

	mp.SetTranscodeLimit(6)
	assert.Equal(t, 6, cap(mp.transcode.limit))

	mp.SetTranscodeLimit(0)
	assert.Equal(t, 2, cap(mp.transcode.limit))

	mp.SetTranscodeLimit(-1)
	assert.Equal(t, 2, cap(mp.transcode.limit))
}

func TestKillAllTranscodeProcessesNoPanic(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	assert.NotPanics(t, func() {
		mp.KillAllTranscodeProcesses()
	})
}

func TestKillAllTranscodeProcessesCleanup(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	mp.transcode.mu.Lock()
	mp.transcode.active = make(map[*exec.Cmd]struct{})
	mp.transcode.mu.Unlock()

	mp.KillAllTranscodeProcesses()

	mp.transcode.mu.Lock()
	assert.Empty(t, mp.transcode.active)
	mp.transcode.mu.Unlock()
}

func TestRemoveProcess(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	mp.transcode.active = make(map[*exec.Cmd]struct{})

	cmd := &exec.Cmd{}
	mp.transcode.active[cmd] = struct{}{}
	assert.Len(t, mp.transcode.active, 1)

	mp.removeProcess(cmd)
	assert.Empty(t, mp.transcode.active)

	mp.removeProcess(cmd)
	assert.Empty(t, mp.transcode.active)
}

func createTestVideo(outputPath string) *exec.Cmd {
	return exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=1",
		"-pix_fmt", "yuv420p",
		"-y",
		outputPath,
	)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
