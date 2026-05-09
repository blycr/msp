package media

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSetProbeCacheTTL(t *testing.T) {
	origTTL := cacheTTL
	defer func() { cacheTTL = origTTL }()

	SetProbeCacheTTL(10 * time.Minute)
	assert.Equal(t, 10*time.Minute, cacheTTL)

	SetProbeCacheTTL(1 * time.Second)
	assert.Equal(t, 1*time.Second, cacheTTL)
}

func TestClearProbeCache(t *testing.T) {
	probeCache.Store("test-key", probeCacheEntry{
		info:   CodecInfo{VideoCodec: "h264"},
		mtime:  12345,
		expire: time.Now().Add(5 * time.Minute),
	})

	ClearProbeCache()

	_, ok := probeCache.Load("test-key")
	assert.False(t, ok, "缓存应该被清除")
}

func TestGetCacheKey(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		mtime int64
	}{
		{"普通路径", "/path/to/file.mp4", 1234567890},
		{"空路径", "", 0},
		{"带特殊字符", "/path/with spaces/file.mp4", 9999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := getCacheKey(tt.path, tt.mtime)
			assert.Contains(t, key, tt.path)
		})
	}
}

func TestGetFileMtime(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	assert.NoError(t, os.WriteFile(testFile, []byte("test"), 0600))

	mtime := getFileMtime(testFile)
	assert.Greater(t, mtime, int64(0))

	assert.Equal(t, int64(0), getFileMtime("/nonexistent/file"))
}

func TestProbeCacheExpiry(t *testing.T) {
	origTTL := cacheTTL
	defer func() { cacheTTL = origTTL }()

	SetProbeCacheTTL(100 * time.Millisecond)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	assert.NoError(t, os.WriteFile(testFile, []byte("fake"), 0600))

	mtime := getFileMtime(testFile)
	cacheKey := getCacheKey(testFile, mtime)

	probeCache.Store(cacheKey, probeCacheEntry{
		info:   CodecInfo{VideoCodec: "cached"},
		mtime:  mtime,
		expire: time.Now().Add(100 * time.Millisecond),
	})

	cached, ok := probeCache.Load(cacheKey)
	assert.True(t, ok)
	entry := cached.(probeCacheEntry)
	assert.Equal(t, "cached", entry.info.VideoCodec)

	time.Sleep(150 * time.Millisecond)

	cached, ok = probeCache.Load(cacheKey)
	if ok {
		entry = cached.(probeCacheEntry)
		assert.True(t, time.Now().After(entry.expire), "缓存应该已过期")
	}
}

func TestCheckFFmpegWithoutPanic(t *testing.T) {
	ResetPathsForTest()
	defer ResetPathsForTest()

	assert.NotPanics(t, func() {
		_ = CheckFFmpeg()
	})
}

func TestCheckFFprobeWithoutPanic(t *testing.T) {
	ResetPathsForTest()
	defer ResetPathsForTest()

	assert.NotPanics(t, func() {
		_ = CheckFFprobe()
	})
}

func TestFFmpegPathDiscovery(t *testing.T) {
	ResetPathsForTest()
	defer ResetPathsForTest()

	// Test that FFmpegPath does not panic
	assert.NotPanics(t, func() {
		_ = FFmpegPath()
	})

	// Test that FFprobePath does not panic
	assert.NotPanics(t, func() {
		_ = FFprobePath()
	})

	// Test FFmpegAvailable consistency
	path := FFmpegPath()
	assert.Equal(t, path != "", FFmpegAvailable())
}

func TestExeName(t *testing.T) {
	name := exeName("ffmpeg")
	if runtime.GOOS == "windows" {
		assert.Equal(t, "ffmpeg.exe", name)
	} else {
		assert.Equal(t, "ffmpeg", name)
	}
}

func TestLocalCandidatePaths(t *testing.T) {
	paths := localCandidatePaths("ffmpeg")
	assert.NotEmpty(t, paths)

	// Should contain at least executable dir and cwd candidates
	foundExe := false
	foundCwd := false
	for _, p := range paths {
		if filepath.Base(p) == "ffmpeg" || filepath.Base(p) == "ffmpeg.exe" {
			// Check if it's from the executable directory or cwd
			if strings.Contains(p, "bin") {
				foundExe = true
			} else {
				foundCwd = true
			}
		}
	}
	// At least cwd paths should exist (the test binary runs from somewhere)
	assert.True(t, foundCwd || foundExe, "should contain at least one candidate path")
}

func TestPlatformCandidatePaths(t *testing.T) {
	paths := platformCandidatePaths("ffmpeg")
	assert.NotEmpty(t, paths)

	// All paths should end with the executable name
	for _, p := range paths {
		assert.True(t,
			strings.HasSuffix(p, "ffmpeg") || strings.HasSuffix(p, "ffmpeg.exe"),
			"platform path should end with executable name: %s", p)
	}
}

func TestGetCodecInfoWithFakeFile(t *testing.T) {
	if !CheckFFprobe() {
		t.Skip("FFprobe 未安装，跳过测试")
	}

	tmpDir := t.TempDir()
	fakeFile := filepath.Join(tmpDir, "fake.mp4")
	assert.NoError(t, os.WriteFile(fakeFile, []byte("not a real video"), 0600))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := GetCodecInfo(ctx, fakeFile)
	if err == nil {
		assert.Empty(t, info.VideoCodec)
		assert.Empty(t, info.AudioCodec)
	}
}

func TestProbeCacheHit(t *testing.T) {
	origTTL := cacheTTL
	defer func() { cacheTTL = origTTL }()
	SetProbeCacheTTL(5 * time.Minute)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "cached.mp4")
	assert.NoError(t, os.WriteFile(testFile, []byte("fake"), 0600))

	mtime := getFileMtime(testFile)
	cacheKey := getCacheKey(testFile, mtime)

	expected := CodecInfo{VideoCodec: "h264", AudioCodec: "aac"}
	probeCache.Store(cacheKey, probeCacheEntry{
		info:   expected,
		mtime:  mtime,
		expire: time.Now().Add(5 * time.Minute),
	})

	ctx := context.Background()
	info, err := GetCodecInfo(ctx, testFile)
	assert.NoError(t, err)
	assert.Equal(t, expected, info)
}
