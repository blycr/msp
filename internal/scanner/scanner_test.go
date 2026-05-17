package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"msp/internal/config"
	"msp/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		// 视频
		{".mp4", "video"},
		{".MP4", "video"},
		{".mkv", "video"},
		{".MKV", "video"},
		{".mov", "video"},
		{".avi", "video"},
		{".webm", "video"},
		{".m4v", "video"},
		{".wmv", "video"},
		{".WMV", "video"},
		// 音频
		{".mp3", "audio"},
		{".MP3", "audio"},
		{".aac", "audio"},
		{".flac", "audio"},
		{".wav", "audio"},
		{".m4a", "audio"},
		{".ogg", "audio"},
		{".opus", "audio"},
		// 图片
		{".jpg", "image"},
		{".jpeg", "image"},
		{".png", "image"},
		{".gif", "image"},
		{".webp", "image"},
		{".svg", "image"},
		// 其他
		{".txt", "other"},
		{".doc", "other"},
		{"", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := ClassifyExt(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSubtitleExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".vtt", true},
		{".srt", true},
		{".ass", true},
		{".ssa", true},
		{".VTT", true},
		{".mp4", false},
		{".txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := IsSubtitleExt(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLyricsExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".lrc", true},
		{".LRC", false}, // 区分大小写
		{".txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := IsLyricsExt(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsBlockedString(t *testing.T) {
	tests := []struct {
		name     string
		list     []string
		target   string
		expected bool
	}{
		{
			name:     "精确匹配",
			list:     []string{"test.txt"},
			target:   "test.txt",
			expected: true,
		},
		{
			name:     "大小写不敏感匹配",
			list:     []string{"TEST.TXT"},
			target:   "test.txt",
			expected: true,
		},
		{
			name:     "正则匹配",
			list:     []string{"/^test.*\\.txt$/"},
			target:   "test123.txt",
			expected: true,
		},
		{
			name:     "不匹配",
			list:     []string{"other.txt"},
			target:   "test.txt",
			expected: false,
		},
		{
			name:     "空列表",
			list:     []string{},
			target:   "test.txt",
			expected: false,
		},
		{
			name:     "列表中有空字符串",
			list:     []string{"", "test.txt"},
			target:   "test.txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBlockedString(tt.list, tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsBlockedSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		rule     string
		expected bool
	}{
		{
			name:     "范围匹配",
			size:     500,
			rule:     "100-1000",
			expected: true,
		},
		{
			name:     "范围不匹配",
			size:     50,
			rule:     "100-1000",
			expected: false,
		},
		{
			name:     "大于等于",
			size:     1000,
			rule:     ">=1000",
			expected: true,
		},
		{
			name:     "小于等于",
			size:     500,
			rule:     "<=1000",
			expected: true,
		},
		{
			name:     "大于",
			size:     1001,
			rule:     ">1000",
			expected: true,
		},
		{
			name:     "小于",
			size:     999,
			rule:     "<1000",
			expected: true,
		},
		{
			name:     "带单位",
			size:     1024 * 1024,
			rule:     ">=1MB",
			expected: true,
		},
		{
			name:     "空规则",
			size:     1000,
			rule:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBlockedSize(tt.size, tt.rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubtitleLabel(t *testing.T) {
	tests := []struct {
		token    string
		expected string
	}{
		{"zh", "中文"},
		{"zh-cn", "中文"},
		{"en", "English"},
		{"eng", "English"},
		{"ja", "日本語"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			result := SubtitleLabel(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSrtToVtt(t *testing.T) {
	srt := `1
00:00:01,000 --> 00:00:04,000
Hello World

2
00:00:05,000 --> 00:00:08,000
Second subtitle
with multiple lines
`

	vtt := SrtToVtt([]byte(srt))
	vttStr := string(vtt)

	assert.Contains(t, vttStr, "WEBVTT")
	assert.Contains(t, vttStr, "00:00:01.000 --> 00:00:04.000")
	assert.Contains(t, vttStr, "Hello World")
	assert.NotContains(t, vttStr, "00:00:01,000") // 逗号应该被替换为点
}

func TestAssToVtt(t *testing.T) {
	ass := `[Script Info]
Title: Test

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Hello World
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,Second line
`

	vtt := AssToVtt([]byte(ass))
	vttStr := string(vtt)

	assert.Contains(t, vttStr, "WEBVTT")
	assert.Contains(t, vttStr, "00:00:01.00")
	assert.Contains(t, vttStr, "Hello World")
}

func TestIsAllDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"123abc", false},
		{"", false},
		{"12 3", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsAllDigits(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWalkShares(t *testing.T) {
	// 创建临时目录结构
	tmpDir := t.TempDir()

	// 创建测试文件
	videoDir := filepath.Join(tmpDir, "videos")
	audioDir := filepath.Join(tmpDir, "music")
	require.NoError(t, os.MkdirAll(videoDir, 0750))
	require.NoError(t, os.MkdirAll(audioDir, 0750))

	// 创建媒体文件
	require.NoError(t, os.WriteFile(filepath.Join(videoDir, "movie.mp4"), []byte("fake video"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(videoDir, "show.mkv"), []byte("fake video"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(audioDir, "song.mp3"), []byte("fake audio"), 0600))

	// 创建字幕文件
	require.NoError(t, os.WriteFile(filepath.Join(videoDir, "movie.zh.srt"), []byte("1\n00:00:01,000 --> 00:00:04,000\nHello"), 0600))

	// 创建隐藏目录和文件（应该被跳过）
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	require.NoError(t, os.MkdirAll(hiddenDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "secret.mp4"), []byte("secret"), 0600))

	shares := []domain.Share{
		{Path: videoDir, Label: "Videos"},
		{Path: audioDir, Label: "Music"},
	}

	blacklist := config.BlacklistConfig{}

	t.Run("遍历所有共享目录", func(t *testing.T) {
		var items []domain.MediaItem
		cb := func(item domain.MediaItem, path string, root string) error {
			items = append(items, item)
			return nil
		}

		err := WalkShares(context.Background(), shares, blacklist, 0, cb, nil)
		require.NoError(t, err)

		// 应该有 3 个媒体文件（2 视频 + 1 音频）
		assert.Len(t, items, 3)

		// 验证隐藏目录的文件没有被包含
		for _, item := range items {
			assert.NotContains(t, item.Path, ".hidden")
		}
	})

	t.Run("限制数量", func(t *testing.T) {
		var items []domain.MediaItem
		cb := func(item domain.MediaItem, path string, root string) error {
			items = append(items, item)
			return nil
		}

		err := WalkShares(context.Background(), shares, blacklist, 2, cb, nil)
		require.NoError(t, err)

		// 应该只有 2 个
		assert.Len(t, items, 2)
	})

	t.Run("上下文取消", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		var count int
		cb := func(item domain.MediaItem, path string, root string) error {
			count++
			if count >= 1 {
				cancel() // 取消上下文
			}
			return nil
		}

		err := WalkShares(ctx, shares, blacklist, 0, cb, nil)
		// 应该返回上下文取消错误
		assert.Error(t, err)
	})
}

func TestFindSidecarSubtitles(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建视频文件
	videoPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("fake"), 0600))

	// 创建字幕文件
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.zh.srt"), []byte("subtitle"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.en.srt"), []byte("subtitle"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "other.srt"), []byte("subtitle"), 0600))

	subs := FindSidecarSubtitles(videoPath, nil)

	// 应该找到 2 个字幕（zh 和 en）
	assert.Len(t, subs, 2)

	// 中文应该排在前面
	if len(subs) >= 2 {
		assert.Equal(t, "zh", subs[0].Lang)
		assert.True(t, subs[0].Default)
	}
}

func TestFindAudioSidecars(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建音频文件
	audioPath := filepath.Join(tmpDir, "song.mp3")
	require.NoError(t, os.WriteFile(audioPath, []byte("fake"), 0600))

	// 创建歌词文件
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "song.lrc"), []byte("[00:00.00]Lyrics"), 0600))

	// 创建封面文件
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "song.jpg"), []byte("fake image"), 0600))

	cache := make(map[string][]os.DirEntry)
	cover, lyrics := FindAudioSidecarsCached(audioPath, cache)

	assert.NotEmpty(t, cover)
	assert.NotEmpty(t, lyrics)
	assert.Contains(t, cover, "song.jpg")
	assert.Contains(t, lyrics, "song.lrc")
}

func TestSniffContainerCodecs(t *testing.T) {
	// 创建假的 MKV 文件（包含特征码）
	tmpDir := t.TempDir()
	mkvPath := filepath.Join(tmpDir, "test.mkv")

	// 写入 MKV 头部和特征码
	data := []byte("\x1a\x45\xdf\xa3")                // EBML 头部
	data = append(data, []byte("V_MPEG4/ISO/AVC")...) // H.264
	data = append(data, []byte("A_AAC")...)           // AAC
	require.NoError(t, os.WriteFile(mkvPath, data, 0600))

	video, audio := SniffContainerCodecs(mkvPath, ".mkv")

	assert.Contains(t, video, "H.264")
	assert.Contains(t, audio, "AAC")
}

func TestNormalizeBaseForMatch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Movie.2024.1080p.BluRay.x264",
			expected: "movie.2024", // 年份不会被移除
		},
		{
			input:    "TV.Show.S01E01.720p.WEB-DL",
			expected: "tv.show.s01e01",
		},
		{
			input:    "Simple Movie",
			expected: "simple movie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeBaseForMatch(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildMediaItem(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试文件
	videoPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("fake"), 0600))

	// 创建字幕文件
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.zh.srt"), []byte("subtitle"), 0600))

	// 获取文件信息
	info, err := os.Stat(videoPath)
	require.NoError(t, err)

	// 创建 DirEntry
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	var videoEntry os.DirEntry
	for _, e := range entries {
		if e.Name() == "movie.mp4" {
			videoEntry = e
			break
		}
	}
	require.NotNil(t, videoEntry)

	w := &shareWalker{idCodec: nil}
	item, err := w.buildMediaItem(videoPath, videoEntry, "Videos", tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "movie.mp4", item.Name)
	assert.Equal(t, ".mp4", item.Ext)
	assert.Equal(t, "video", item.Kind)
	assert.Equal(t, "Videos", item.ShareLabel)
	assert.Equal(t, info.Size(), item.Size)
	assert.NotEmpty(t, item.ID)
	assert.NotEmpty(t, item.Subtitles)
}

// 基准测试

func BenchmarkClassifyExt(b *testing.B) {
	exts := []string{".mp4", ".MP4", ".mkv", ".avi", ".mp3", ".flac", ".jpg", ".png"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ext := range exts {
			_ = ClassifyExt(ext)
		}
	}
}

func BenchmarkIsBlockedString(b *testing.B) {
	list := []string{"test.txt", "/^prefix.*/", "exact.match"}
	target := "prefix_test_file.txt"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsBlockedString(list, target)
	}
}

func BenchmarkSrtToVtt(b *testing.B) {
	srt := []byte(`1
00:00:01,000 --> 00:00:04,000
Hello World

2
00:00:05,000 --> 00:00:08,000
Second subtitle
`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SrtToVtt(srt)
	}
}

func TestSrtToVttBOM(t *testing.T) {
	srt := []byte{0xEF, 0xBB, 0xBF}
	srt = append(srt, []byte(`1
00:00:01,000 --> 00:00:04,000
Hello BOM

`)...)

	vtt := SrtToVtt(srt)
	vttStr := string(vtt)

	assert.Contains(t, vttStr, "WEBVTT")
	assert.Contains(t, vttStr, "Hello BOM")
	assert.Contains(t, vttStr, "00:00:01.000")
}

func TestSrtToVttMultipleSubtitles(t *testing.T) {
	srt := `1
00:00:01,000 --> 00:00:04,000
First line

2
00:00:05,500 --> 00:00:08,300
Second line

3
00:00:10,000 --> 00:00:15,000
Third line
with multiple lines
`

	vtt := SrtToVtt([]byte(srt))
	vttStr := string(vtt)

	assert.Contains(t, vttStr, "WEBVTT")
	assert.Contains(t, vttStr, "First line")
	assert.Contains(t, vttStr, "Second line")
	assert.Contains(t, vttStr, "Third line")
	assert.Contains(t, vttStr, "00:00:05.500 --> 00:00:08.300")
	assert.NotContains(t, vttStr, "1\n")
	assert.NotContains(t, vttStr, "2\n")
}

func TestAssToVttWithStyles(t *testing.T) {
	ass := `[Script Info]
Title: Test ASS
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:01:30.50,0:01:35.25,Default,,0,0,0,,Hello from ASS
Dialogue: 0,0:02:00.00,0:02:05.00,Default,,0,0,0,,Second ASS line
`

	vtt := AssToVtt([]byte(ass))
	vttStr := string(vtt)

	assert.Contains(t, vttStr, "WEBVTT")
	assert.Contains(t, vttStr, "Hello from ASS")
	assert.Contains(t, vttStr, "Second ASS line")
	assert.Contains(t, vttStr, "00:01:30.500")
	assert.Contains(t, vttStr, "00:02:00.000")
}

func TestAssToVttWithOverrideTags(t *testing.T) {
	ass := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,{\b1}Bold text{\b0}
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,Normal{\i1}Italic{\i0} text
`

	vtt := AssToVtt([]byte(ass))
	vttStr := string(vtt)

	assert.Contains(t, vttStr, "WEBVTT")
	assert.Contains(t, vttStr, "Bold text")
	assert.NotContains(t, vttStr, "{\\b1}")
	assert.Contains(t, vttStr, "Normal")
	assert.Contains(t, vttStr, "Italic")
}

func TestAssToVttWithNewlines(t *testing.T) {
	ass := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Line one\NLine two\nLine three
`

	vtt := AssToVtt([]byte(ass))
	vttStr := string(vtt)

	assert.Contains(t, vttStr, "WEBVTT")
	assert.Contains(t, vttStr, "Line one")
	assert.Contains(t, vttStr, "Line two")
	assert.Contains(t, vttStr, "Line three")
}

func TestFindSidecarSubtitlesCached(t *testing.T) {
	tmpDir := t.TempDir()

	videoPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("fake"), 0600))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.zh.srt"), []byte("zh sub"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.en.srt"), []byte("en sub"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.default.vtt"), []byte("WEBVTT\n\n00:00:01.000 --> 00:00:04.000\nDefault"), 0600))

	cache := make(map[string][]os.DirEntry)

	subs := FindSidecarSubtitlesCached(videoPath, cache, nil)
	assert.Len(t, subs, 3)

	assert.True(t, cache[tmpDir] != nil)

	subs2 := FindSidecarSubtitlesCached(videoPath, cache, nil)
	assert.Len(t, subs2, 3)
}

func TestFindSidecarSubtitlesNoMatch(t *testing.T) {
	tmpDir := t.TempDir()

	videoPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("fake"), 0600))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "other.zh.srt"), []byte("not matching"), 0600))

	subs := FindSidecarSubtitles(videoPath, nil)
	assert.Nil(t, subs)
}

func TestExtractBaseVariants(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		minCount int
	}{
		{"简单名称", "movie", 1},
		{"带编码信息", "Movie.1080p.BluRay.x264", 2},
		{"带年份", "Movie.2024.720p.WEB-DL", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variants := extractBaseVariants(tt.base)
			assert.GreaterOrEqual(t, len(variants), tt.minCount)
			assert.Equal(t, strings.ToLower(tt.base), variants[0])
		})
	}
}

func TestSortSubtitles(t *testing.T) {
	subs := []domain.Subtitle{
		{Lang: "en", Label: "English"},
		{Lang: "zh", Label: "中文"},
		{Lang: "ja", Label: "日本語"},
	}

	sortSubtitles(subs)

	assert.Equal(t, "zh", subs[0].Lang, "中文应该排在第一位")
}

func TestLrcPicker(t *testing.T) {
	t.Run("精确匹配优先", func(t *testing.T) {
		p := &lrcPicker{}
		p.consider("song.lrc", "song", "song")
		p.consider("song.cn.lrc", "song.cn", "song")
		assert.Equal(t, "song.lrc", p.choose())
	})

	t.Run("语言次之", func(t *testing.T) {
		p := &lrcPicker{}
		p.consider("song.cn.lrc", "song.cn", "song")
		p.consider("other.lrc", "other", "song")
		assert.Equal(t, "song.cn.lrc", p.choose())
	})

	t.Run("任意匹配", func(t *testing.T) {
		p := &lrcPicker{}
		p.consider("other.lrc", "other", "song")
		assert.Equal(t, "other.lrc", p.choose())
	})

	t.Run("无匹配", func(t *testing.T) {
		p := &lrcPicker{}
		assert.Equal(t, "", p.choose())
	})
}
