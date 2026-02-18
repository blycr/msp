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
		after   TranscodeOptions // 验证后的期望状态
	}{
		{
			name:    "默认格式",
			opts:    TranscodeOptions{},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4", Bitrate: "", Offset: 0},
		},
		{
			name:    "有效格式 mp4",
			opts:    TranscodeOptions{Format: "mp4"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4"},
		},
		{
			name:    "有效格式 mp3",
			opts:    TranscodeOptions{Format: "mp3"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp3"},
		},
		{
			name:    "有效格式 aac",
			opts:    TranscodeOptions{Format: "AAC"}, // 大写也应该被接受
			wantErr: false,
			after:   TranscodeOptions{Format: "aac"},
		},
		{
			name:    "无效格式",
			opts:    TranscodeOptions{Format: "exe"},
			wantErr: true,
		},
		{
			name:    "有效码率",
			opts:    TranscodeOptions{Format: "mp4", Bitrate: "2M"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4", Bitrate: "2m"},
		},
		{
			name:    "有效码率 k",
			opts:    TranscodeOptions{Format: "mp3", Bitrate: "128k"},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp3", Bitrate: "128k"},
		},
		{
			name:    "无效码率格式",
			opts:    TranscodeOptions{Format: "mp4", Bitrate: "2M;rm -rf /"},
			wantErr: true,
		},
		{
			name:    "负偏移量",
			opts:    TranscodeOptions{Format: "mp4", Offset: -10},
			wantErr: false,
			after:   TranscodeOptions{Format: "mp4", Offset: 0},
		},
		{
			name:    "正常偏移量",
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
	// 这个测试依赖于系统是否安装了 FFmpeg
	// 我们只测试函数不会 panic
	result := CheckFFmpeg()
	// 结果可能是 true 或 false，取决于环境
	_ = result
}

func TestCheckFFprobe(t *testing.T) {
	// 这个测试依赖于系统是否安装了 FFprobe
	result := CheckFFprobe()
	_ = result
}

func TestGetCodecInfo(t *testing.T) {
	if !CheckFFprobe() {
		t.Skip("FFprobe 未安装，跳过测试")
	}

	// 创建一个假的 MP4 文件
	tmpDir := t.TempDir()
	fakeMP4 := filepath.Join(tmpDir, "test.mp4")

	// 写入一个最小的 MP4 文件头（ftyp 盒子）
	// ftyp 盒子结构：大小(4字节) + "ftyp"(4字节) + 品牌(4字节) + 版本(4字节)
	mp4Header := []byte{
		0x00, 0x00, 0x00, 0x18, // 盒子大小 24
		'f', 't', 'y', 'p', // 盒子类型
		'i', 's', 'o', 'm', // 主要品牌
		0x00, 0x00, 0x00, 0x00, // 次要版本
		'i', 's', 'o', 'm', // 兼容品牌
	}
	require.NoError(t, os.WriteFile(fakeMP4, mp4Header, 0600))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := GetCodecInfo(ctx, fakeMP4)
	// 对于假文件，FFprobe 可能返回错误或空结果
	// 我们主要测试函数不会 panic
	_ = err
	_ = info
}

func TestTranscodeStreamValidation(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("目录而不是文件", func(t *testing.T) {
		opts := TranscodeOptions{Format: "mp4"}
		_, err := TranscodeStream(context.Background(), tmpDir, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory")
	})

	t.Run("不存在的文件", func(t *testing.T) {
		opts := TranscodeOptions{Format: "mp4"}
		_, err := TranscodeStream(context.Background(), "/nonexistent/file.mp4", opts)
		assert.Error(t, err)
	})

	t.Run("无效格式", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.mp4")
		require.NoError(t, os.WriteFile(testFile, []byte("fake"), 0600))

		opts := TranscodeOptions{Format: "invalid"}
		_, err := TranscodeStream(context.Background(), testFile, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid options")
	})

	t.Run("非普通文件（符号链接）", func(t *testing.T) {
		// Windows 上符号链接的行为与 Unix 不同
		// 在 Windows 上，os.Symlink 创建的链接可能被报告为常规文件
		// 这个测试在 Unix 系统上运行
		if os.PathSeparator == '\\' {
			t.Skip("跳过 Windows 符号链接测试")
		}

		targetFile := filepath.Join(tmpDir, "target.mp4")
		linkFile := filepath.Join(tmpDir, "link.mp4")
		require.NoError(t, os.WriteFile(targetFile, []byte("fake"), 0600))
		require.NoError(t, os.Symlink(targetFile, linkFile))

		opts := TranscodeOptions{Format: "mp4"}
		_, err := TranscodeStream(context.Background(), linkFile, opts)
		// 符号链接应该被拒绝
		assert.Error(t, err)
	})
}

func TestTranscodeStreamConcurrency(t *testing.T) {
	if !CheckFFmpeg() {
		t.Skip("FFmpeg 未安装，跳过测试")
	}

	// 创建一个测试视频文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")

	// 使用 FFmpeg 创建一个测试视频
	cmd := createTestVideo(testFile)
	if cmd != nil {
		err := cmd.Run()
		if err != nil {
			t.Skip("无法创建测试视频:", err)
		}
	}

	// 测试并发限制
	ctx := context.Background()
	opts := TranscodeOptions{Format: "mp4"}

	// 启动多个转码请求，应该受到限制
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			stream, err := TranscodeStream(ctx, testFile, opts)
			if err == nil && stream != nil {
				// 读取并关闭流
				_, _ = io.Copy(io.Discard, stream)
				_ = stream.Close()
			}
			done <- err
		}()
	}

	// 等待所有请求完成
	var busyCount int
	for i := 0; i < 5; i++ {
		err := <-done
		if err != nil && contains(err.Error(), "busy") {
			busyCount++
		}
	}

	// 应该有部分请求因为并发限制而失败
	assert.Greater(t, busyCount, 0, "应该有请求因为并发限制而失败")
}

func TestLimitReleaser(t *testing.T) {
	// 重置信号量用于测试
	transcodeLimit = make(chan struct{}, 2)

	// 获取信号量
	transcodeLimit <- struct{}{}
	assert.Equal(t, 1, len(transcodeLimit))

	// 创建 limitReleaser
	pr, pw := io.Pipe()
	lr := &limitReleaser{ReadCloser: pr}

	// 关闭应该释放信号量
	err := lr.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(transcodeLimit))

	// 多次关闭应该安全（只释放一次）
	err = lr.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(transcodeLimit))

	_ = pw.Close()
}

func TestSetTranscodeLimit(t *testing.T) {
	// Save and restore original
	origLimit := transcodeLimit
	defer func() { transcodeLimit = origLimit }()

	SetTranscodeLimit(6)
	assert.Equal(t, 6, cap(transcodeLimit))

	// Zero should default to 2
	SetTranscodeLimit(0)
	assert.Equal(t, 2, cap(transcodeLimit))

	// Negative should default to 2
	SetTranscodeLimit(-1)
	assert.Equal(t, 2, cap(transcodeLimit))
}

// 辅助函数

func createTestVideo(outputPath string) *exec.Cmd {
	// 使用 FFmpeg 创建一个 1 秒的测试视频
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
