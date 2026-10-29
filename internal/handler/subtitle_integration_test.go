package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"msp/internal/media"
	"msp/internal/util"
)

// TestHandleSubtitleEmbeddedRealFile 校验内嵌字幕按全局 stream index 提取。
//
// 回归目标：serveEmbeddedSubtitle 曾用 "-map 0:s:<n>"，而 /api/probe 下发的
// subtitle.id 是 ffprobe 的**全局** stream index。两者语义不同：
//   - 单/双字幕文件：0:s:2 匹配不到任何流 → ffmpeg 退出非零 → 404；
//   - 三条以上字幕：能出内容，但是第 (n+1) 条，即**取错轨**。
//
// 这里用「video=0, audio=1, subtitle=2」的文件，请求 track=2 必须拿到字幕正文。
func TestHandleSubtitleEmbeddedRealFile(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed; skipping subtitle integration test")
	}

	dir := t.TempDir()
	srt := filepath.Join(dir, "s.srt")
	if err := os.WriteFile(srt,
		[]byte("1\n00:00:01,000 --> 00:00:03,000\nhello-embedded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(dir, "v.mkv")
	args := []string{
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=160x120:rate=5:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-i", srt,
		"-map", "0:v", "-map", "1:a", "-map", "2:s",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac", "-c:s", "srt",
		video,
	}
	cmd := exec.Command(bin, args...) //nolint:gosec // 参数来自测试内部，非外部输入
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg 生成样本失败: %v\n%s", err, b)
	}

	s := newTestServerWithShare(t, dir)
	idCodec := util.NewIDCodec(nil)
	h := New(Deps{
		Config:    s,
		Media:     s,
		Session:   s,
		Logger:    s,
		Processor: media.NewMediaProcessor(nil, idCodec),
		IDCodec:   idCodec,
	})

	id := idCodec.EncodeID(video)
	req := httptest.NewRequest(http.MethodGet, "/api/subtitle?id="+id+"&track=2", nil)
	w := httptest.NewRecorder()

	h.HandleSubtitle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("track=2（字幕的全局 index）应返回 200，实际 %d，body=%q", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "hello-embedded") {
		t.Errorf("响应里没有字幕正文，body=%q", body)
	}
	if !strings.HasPrefix(body, "WEBVTT") {
		t.Errorf("响应不是 WebVTT，body=%q", body)
	}
}
