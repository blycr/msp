package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/util"
)

func TestDetermineContentType(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".mp4", "video/mp4"},
		{".mkv", "video/x-matroska"},
		{".webm", "video/webm"},
		{".avi", "video/x-msvideo"},
		{".wmv", "video/x-ms-wmv"},
		{".mov", "video/quicktime"},
		{".ts", "video/mp2t"},
		{".vtt", "text/vtt; charset=utf-8"},
		{".srt", "text/plain; charset=utf-8"},
		{".lrc", "text/plain; charset=utf-8"},
		{".m4v", "video/mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := determineContentType(tt.ext)
			if got != tt.want {
				t.Errorf("determineContentType(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestHandleProbeMissingID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/probe", nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleProbeBadID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/probe?id=!!!invalid!!!", nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleProbeForbiddenFile(t *testing.T) {
	h, _ := setupTestHandler(t)

	target := filepath.Join(t.TempDir(), "test.mp4")
	_ = os.WriteFile(target, []byte("fake"), 0600)
	id := util.EncodeID(target)

	req := httptest.NewRequest(http.MethodGet, "/api/probe?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for file not in shares, got %d", w.Code)
	}
}

func TestHandleStreamMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/stream", nil)
	w := httptest.NewRecorder()

	h.HandleStream(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleStreamMissingID(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
	w := httptest.NewRecorder()

	h.HandleStream(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleProbeMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/probe", nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleProbeWithValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	s := newTestServerWithShare(t, configPath, tmpDir)

	h := New(Deps{Config: s, Media: s, Session: s, Logger: s, Progress: nil, Prefs: nil})

	videoPath := filepath.Join(tmpDir, "test.mp4")
	_ = os.WriteFile(videoPath, []byte("fake-mp4-data"), 0600)
	id := util.EncodeID(videoPath)

	req := httptest.NewRequest(http.MethodGet, "/api/probe?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleProbe(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleStreamForbiddenFile(t *testing.T) {
	h, _ := setupTestHandler(t)

	target := filepath.Join(t.TempDir(), "test.mp4")
	_ = os.WriteFile(target, []byte("fake"), 0600)
	id := util.EncodeID(target)

	req := httptest.NewRequest(http.MethodGet, "/api/stream?id="+id, nil)
	w := httptest.NewRecorder()

	h.HandleStream(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for file not in shares, got %d", w.Code)
	}
}

func TestDecidePlaybackMode(t *testing.T) {
	tests := []struct {
		name        string
		videoCodec  string
		audioCodec  string
		ffmpegAvail bool
		wantMode    string
	}{
		// FFmpeg 不可用
		{name: "no ffmpeg", videoCodec: "H.264/AVC", audioCodec: "AAC", ffmpegAvail: false, wantMode: "direct"},
		{name: "no ffmpeg hevc", videoCodec: "H.265/HEVC", audioCodec: "AC-3", ffmpegAvail: false, wantMode: "direct"},

		// H.264 视频 — 直连
		{name: "h264+aac", videoCodec: "H.264/AVC", audioCodec: "AAC", ffmpegAvail: true, wantMode: "direct"},
		{name: "h264 lower", videoCodec: "h264", audioCodec: "aac", ffmpegAvail: true, wantMode: "direct"},
		{name: "avc1", videoCodec: "avc1", audioCodec: "AAC", ffmpegAvail: true, wantMode: "direct"},
		{name: "h264+mp3", videoCodec: "H.264/AVC", audioCodec: "MP3", ffmpegAvail: true, wantMode: "direct"},
		{name: "h264+opus", videoCodec: "H.264/AVC", audioCodec: "Opus", ffmpegAvail: true, wantMode: "direct"},
		{name: "h264+vorbis", videoCodec: "H.264/AVC", audioCodec: "Vorbis", ffmpegAvail: true, wantMode: "direct"},
		{name: "h264+flac", videoCodec: "H.264/AVC", audioCodec: "FLAC", ffmpegAvail: true, wantMode: "direct"},

		// HEVC 视频 — 转码
		{name: "hevc+aac mkv", videoCodec: "H.265/HEVC", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},
		{name: "hevc+aac mp4", videoCodec: "H.265/HEVC", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},
		{name: "hevc lower", videoCodec: "hevc", audioCodec: "aac", ffmpegAvail: true, wantMode: "transcode"},
		{name: "hvc1", videoCodec: "hvc1", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},
		{name: "hev1", videoCodec: "hev1", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},

		// AV1 视频 — 转码
		{name: "av1+aac", videoCodec: "AV1", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},
		{name: "av01", videoCodec: "av01", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},

		// VP9 视频 — 转码（保守策略）
		{name: "vp9+opus", videoCodec: "VP9", audioCodec: "Opus", ffmpegAvail: true, wantMode: "transcode"},

		// VC-1 视频 — 转码
		{name: "vc1", videoCodec: "VC-1", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},
		{name: "wmv3", videoCodec: "wmv3", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},

		// AC-3 音频 — 转码（解决"有画无声"）
		{name: "h264+ac3", videoCodec: "H.264/AVC", audioCodec: "AC-3", ffmpegAvail: true, wantMode: "transcode"},
		{name: "h264+eac3", videoCodec: "H.264/AVC", audioCodec: "E-AC-3", ffmpegAvail: true, wantMode: "transcode"},
		{name: "h264+ac3 lower", videoCodec: "H.264/AVC", audioCodec: "ac-3", ffmpegAvail: true, wantMode: "transcode"},

		// DTS/TrueHD 音频 — 转码
		{name: "h264+dts", videoCodec: "H.264/AVC", audioCodec: "DTS", ffmpegAvail: true, wantMode: "transcode"},
		{name: "h264+dca", videoCodec: "H.264/AVC", audioCodec: "dca", ffmpegAvail: true, wantMode: "transcode"},
		{name: "h264+truehd", videoCodec: "H.264/AVC", audioCodec: "TrueHD", ffmpegAvail: true, wantMode: "transcode"},

		// 无编码信息 — 直连
		{name: "empty codecs", videoCodec: "", audioCodec: "", ffmpegAvail: true, wantMode: "direct"},

		// 未知编码 — 保守转码
		{name: "unknown video", videoCodec: "prores", audioCodec: "AAC", ffmpegAvail: true, wantMode: "transcode"},
		{name: "unknown audio", videoCodec: "H.264/AVC", audioCodec: "speex", ffmpegAvail: true, wantMode: "transcode"},

		// 嗅探标签格式（字节嗅探返回的 display labels）
		{name: "sniff h264 label", videoCodec: "H.264/AVC", audioCodec: "AAC/MP4A", ffmpegAvail: true, wantMode: "direct"},
		{name: "sniff hevc label", videoCodec: "H.265/HEVC", audioCodec: "E-AC-3", ffmpegAvail: true, wantMode: "transcode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decidePlaybackMode(tt.videoCodec, tt.audioCodec, tt.ffmpegAvail)
			if got != tt.wantMode {
				t.Errorf("decidePlaybackMode(%q, %q, %v) = %q, want %q",
					tt.videoCodec, tt.audioCodec, tt.ffmpegAvail, got, tt.wantMode)
			}
		})
	}
}

func newTestServerWithShare(t *testing.T, configPath, shareDir string) *testServerWrapper {
	t.Helper()
	s := &testServerWrapper{cfg: config.Default()}
	s.cfg.Shares = []domain.Share{{Label: "Test", Path: shareDir}}
	return s
}

type testServerWrapper struct {
	cfg config.Config
}

func (s *testServerWrapper) Config() config.Config {
	return s.cfg
}

func (s *testServerWrapper) UpdateConfig(fn func(*config.Config)) error {
	fn(&s.cfg)
	return nil
}

func (s *testServerWrapper) GetPort() int {
	return s.cfg.Port
}

func (s *testServerWrapper) GetOrBuildMediaCache(_ context.Context, _ []domain.Share, _ config.BlacklistConfig, _ bool) (domain.MediaResponse, string) {
	return domain.MediaResponse{}, ""
}

func (s *testServerWrapper) InvalidateMediaCache() {}

func (s *testServerWrapper) CreateSession() (string, error) {
	return "test-session", nil
}

func (s *testServerWrapper) ValidateSession(_ string) bool {
	return true
}

func (s *testServerWrapper) Log(_ string, _ string) {}

func (s *testServerWrapper) LogRequest(_ *http.Request, _ int, _ time.Time) {}
