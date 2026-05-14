package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"msp/internal/config"
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
		id := util.EncodeID(outsideFile)
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
