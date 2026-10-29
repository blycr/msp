package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mustFFmpeg 返回可用的 ffmpeg 路径；没有则跳过。
// 这类测试需要真实媒体文件，CI 的 ubuntu-latest 自带 ffmpeg。
func mustFFmpeg(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed; skipping media integration test")
	}
	return p
}

// genSample 用 ffmpeg 生成样本，返回文件绝对路径。
func genSample(t *testing.T, name string, args ...string) string {
	t.Helper()
	bin := mustFFmpeg(t)
	out := filepath.Join(t.TempDir(), name)
	full := append([]string{"-y", "-v", "error"}, args...)
	full = append(full, out)
	cmd := exec.Command(bin, full...) //nolint:gosec // 参数来自测试内部，非外部输入
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg 生成样本失败: %v\n%s", err, b)
	}
	return out
}

// TestGetCodecInfoRealFile 用真实文件校验编码与内嵌字幕探测。
//
// 回归目标：probeCodecInfo 曾使用 "-select_streams v:0,a:0,s"。ffprobe 的
// -select_streams 只接受单个 stream specifier，逗号并列会让
// avformat_match_stream_specifier 返回 EINVAL，ffprobe 的 CHECK_END 直接
// goto end，进程以非零码退出 —— 于是 GetCodecInfo 永远返回错误，
// 内嵌字幕永远为空，且转码时 codec 信息为空导致 H.264 也走重编码。
func TestGetCodecInfoRealFile(t *testing.T) {
	srt := filepath.Join(t.TempDir(), "s.srt")
	if err := os.WriteFile(srt,
		[]byte("1\n00:00:01,000 --> 00:00:03,000\nhello\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := genSample(t, "sample.mkv",
		"-f", "lavfi", "-i", "testsrc2=size=160x120:rate=5:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-i", srt,
		"-map", "0:v", "-map", "1:a", "-map", "2:s",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "ac3", "-c:s", "srt",
		"-metadata:s:s:0", "language=chi",
		"-metadata:s:s:0", "title=简体中文",
	)

	mp := NewMediaProcessor(nil, nil)
	info, err := mp.GetCodecInfo(context.Background(), out)
	if err != nil {
		t.Fatalf("GetCodecInfo 返回错误: %v", err)
	}
	if info.VideoCodec != "h264" {
		t.Errorf("VideoCodec = %q, want \"h264\"", info.VideoCodec)
	}
	if info.AudioCodec != "ac3" {
		t.Errorf("AudioCodec = %q, want \"ac3\" (音频流必须能探到)", info.AudioCodec)
	}
	if len(info.Subtitles) != 1 {
		t.Fatalf("内嵌字幕条数 = %d, want 1", len(info.Subtitles))
	}
	sub := info.Subtitles[0]
	if sub.Index != 2 {
		t.Errorf("字幕 Index = %d, want 2（全局 stream index）", sub.Index)
	}
	if sub.Language != "chi" {
		t.Errorf("字幕 Language = %q, want \"chi\"（需用 stream_tags 取）", sub.Language)
	}
	if sub.Title != "简体中文" {
		t.Errorf("字幕 Title = %q, want \"简体中文\"", sub.Title)
	}
}

// TestGetCodecInfoIgnoresAttachedPic 校验封面图不会被当成视频流。
//
// 回归目标：去掉 -select_streams 之后 ffprobe 会返回全部流，带封面的
// .m4a/.mp3 里第一个 video 流是 mjpeg 封面。若不去掉 attached_pic，
// VideoCodec 会变成 "mjpeg"，decidePlaybackMode 会把纯音频判成
// "需要转码"，白白把 AAC 转成有损 MP3。
func TestGetCodecInfoIgnoresAttachedPic(t *testing.T) {
	cover := genSample(t, "cover.jpg",
		"-f", "lavfi", "-i", "color=red:size=64x64:d=1", "-frames:v", "1")

	out := genSample(t, "song.m4a",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-i", cover,
		"-map", "0:a", "-map", "1:v",
		"-c:a", "aac", "-c:v", "mjpeg",
		"-disposition:v", "attached_pic",
	)

	mp := NewMediaProcessor(nil, nil)
	info, err := mp.GetCodecInfo(context.Background(), out)
	if err != nil {
		t.Fatalf("GetCodecInfo 返回错误: %v", err)
	}
	if info.VideoCodec != "" {
		t.Errorf("VideoCodec = %q, want \"\"（封面图必须被忽略）", info.VideoCodec)
	}
	if info.AudioCodec != "aac" {
		t.Errorf("AudioCodec = %q, want \"aac\"", info.AudioCodec)
	}
}
