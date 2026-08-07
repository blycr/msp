package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"msp/internal/config"
	"msp/internal/media"
	"msp/internal/util"
)

func TestDecidePlaybackMode(t *testing.T) {
	tests := []struct {
		name            string
		videoCodec      string
		audioCodec      string
		ffmpegAvailable bool
		want            string
	}{
		{"no_ffmpeg", "h264", "aac", false, "direct"},
		{"h264_aac", "h264", "aac", true, "direct"},
		{"hevc", "hevc", "aac", true, "transcode"},
		{"av1", "av1", "aac", true, "transcode"},
		{"h264_ac3", "h264", "ac-3", true, "transcode"},
		{"h264_dts", "h264", "dts", true, "transcode"},
		{"h264_truehd", "h264", "truehd", true, "transcode"},
		{"h264_mp3", "h264", "mp3", true, "direct"},
		{"h264_opus", "h264", "opus", true, "direct"},
		{"h264_vorbis", "h264", "vorbis", true, "direct"},
		{"h264_flac", "h264", "flac", true, "direct"},
		{"unknown_video", "unknown", "aac", true, "transcode"},
		{"unknown_audio", "h264", "unknown", true, "transcode"},
		{"empty_codecs", "", "", true, "direct"},
		// ffprobe codec_name 原始名边界
		{"h264_eac3", "h264", "eac3", true, "transcode"},
		{"h264_dca", "h264", "dca", true, "transcode"},
		{"vc1", "vc1", "aac", true, "transcode"},
		{"wmv3", "wmv3", "aac", true, "transcode"},
		{"h264_pcm", "h264", "pcm", true, "direct"},
		{"h264_lpcm", "h264", "lpcm", true, "direct"},
		{"h264_wav", "h264", "wav", true, "direct"},
		{"hevc_h265_alias", "h265", "aac", true, "transcode"},
		// 字节嗅探标签路径（应与 ffprobe 原始名决策一致）
		{"sniff_h265", "H.265/HEVC", "AAC", true, "transcode"},
		{"sniff_h264_ac3", "H.264/AVC", "AC-3", true, "transcode"},
		{"sniff_eac3", "H.264/AVC", "E-AC-3", true, "transcode"},
		{"sniff_dts", "H.264/AVC", "DTS", true, "transcode"},
		{"sniff_truehd", "H.264/AVC", "TrueHD", true, "transcode"},
		{"sniff_aac_mp4a", "H.264/AVC", "AAC/MP4A", true, "direct"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decidePlaybackMode(tt.videoCodec, tt.audioCodec, tt.ffmpegAvailable)
			if got != tt.want {
				t.Errorf("decidePlaybackMode(%q, %q, %v) = %q, want %q",
					tt.videoCodec, tt.audioCodec, tt.ffmpegAvailable, got, tt.want)
			}
		})
	}
}

func TestCheckTranscodePolicy(t *testing.T) {
	h, _, _ := setupTestHandlerWithRealServer(t)

	transcodeTrue := true
	transcodeFalse := false

	tests := []struct {
		name       string
		transcode  string
		ext        string
		videoTC    *bool
		audioTC    *bool
		want       bool
		wantErr    bool
		errContain string
	}{
		{"no_transcode_param", "", ".mp4", &transcodeTrue, nil, false, false, ""},
		{"video_allowed", "1", ".mp4", &transcodeTrue, nil, true, false, ""},
		{"video_disabled", "1", ".mp4", &transcodeFalse, nil, false, true, "disabled"},
		{"audio_allowed", "1", ".mp3", nil, &transcodeTrue, true, false, ""},
		{"audio_disabled", "1", ".mp3", nil, &transcodeFalse, false, true, "disabled"},
		{"image_not_allowed", "1", ".jpg", &transcodeTrue, &transcodeTrue, false, true, "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				Playback: config.PlaybackConfig{
					Video: config.PlaybackVideoConfig{Transcode: tt.videoTC},
					Audio: config.PlaybackAudioConfig{Transcode: tt.audioTC},
				},
			}
			r := httptest.NewRequest(http.MethodGet, "/api/stream?transcode="+tt.transcode, nil)
			got, err := h.checkTranscodePolicy(r, cfg, tt.ext)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkTranscodePolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("checkTranscodePolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveMediaTarget(t *testing.T) {
	h, s, tmpDir := setupTestHandlerWithRealServer(t)

	// Create a test media file
	mediaDir := filepath.Join(tmpDir, "media")
	if err := os.MkdirAll(mediaDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	testFile := filepath.Join(mediaDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("fake video"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Add share
	_, _ = h.applySharesOp("add", mediaDir, "Media")
	_ = s.Config().Shares

	t.Run("missing_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
		_, _, _, err := h.resolveMediaTarget(w, r)
		if err == nil {
			t.Error("expected error for missing id")
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("bad_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		// "!!!" is not valid base64
		r := httptest.NewRequest(http.MethodGet, "/api/stream?id=!!!", nil)
		_, _, _, err := h.resolveMediaTarget(w, r)
		if err == nil {
			t.Error("expected error for bad id")
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("not_allowed", func(t *testing.T) {
		w := httptest.NewRecorder()
		outsideFile := filepath.Join(tmpDir, "outside.mp4")
		_ = os.WriteFile(outsideFile, []byte("x"), 0600)
		id := util.NewIDCodec(nil).EncodeID(outsideFile)
		r := httptest.NewRequest(http.MethodGet, "/api/stream?id="+id, nil)
		_, _, _, err := h.resolveMediaTarget(w, r)
		if err == nil {
			t.Error("expected error for not allowed file")
		}
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}

func TestEmbeddedSubtitles(t *testing.T) {
	tracks := []media.SubtitleTrack{
		{Index: 2, CodecName: "subrip", Language: "chi", Title: "简体中文"},
		{Index: 3, CodecName: "ass", Language: "eng"},
		{Index: 4, CodecName: "webvtt"},
	}

	subs := embeddedSubtitles("MEDIA_ID", tracks)
	assert.Len(t, subs, 3)

	// 有 Title 时用 Title
	assert.Equal(t, "简体中文", subs[0].Label)
	assert.Equal(t, "chi", subs[0].Lang)
	assert.Equal(t, "/api/subtitle?id=MEDIA_ID&track=2", subs[0].Src)

	// 无 Title 时用语言标签（scanner.SubtitleLabel）
	assert.Equal(t, "English", subs[1].Label)
	assert.Equal(t, "/api/subtitle?id=MEDIA_ID&track=3", subs[1].Src)

	// 无 Title 且无语言 → 回退 Track N
	assert.Equal(t, "Track 4", subs[2].Label)
	assert.Equal(t, "/api/subtitle?id=MEDIA_ID&track=4", subs[2].Src)
}
