package media

import "testing"

func TestDisplayCodecName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"h264", "H.264/AVC"},
		{"H264", "H.264/AVC"}, // 大小写不敏感
		{"hevc", "H.265/HEVC"},
		{"av1", "AV1"},
		{"vp9", "VP9"},
		{"vc1", "VC-1"},
		{"wmv3", "WMV3"},
		{"aac", "AAC"},
		{"mp3", "MP3"},
		{"opus", "Opus"},
		{"vorbis", "Vorbis"},
		{"flac", "FLAC"},
		{"ac3", "AC-3"},
		{"eac3", "E-AC-3"},
		{"dts", "DTS"},
		{"dca", "DCA"},
		{"truehd", "TrueHD"},
		{"mjpeg", "MJPEG"},          // 未知 → 大写原始名
		{"H.265/HEVC", "H.265/HEVC"}, // 字节嗅探标签透传
		{"TrueHD", "TrueHD"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := DisplayCodecName(tt.in); got != tt.want {
			t.Errorf("DisplayCodecName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
