package media

import "strings"

// codecDisplayNames maps ffprobe codec_name values to human-friendly labels
// that the UI shows in the probe text (e.g. "H.265/HEVC · AC-3").
var codecDisplayNames = map[string]string{
	"h264":   "H.264/AVC",
	"hevc":   "H.265/HEVC",
	"av1":    "AV1",
	"vp9":    "VP9",
	"vc1":    "VC-1",
	"wmv3":   "WMV3",
	"aac":    "AAC",
	"mp3":    "MP3",
	"opus":   "Opus",
	"vorbis": "Vorbis",
	"flac":   "FLAC",
	"pcm":    "PCM",
	"lpcm":   "LPCM",
	"wav":    "WAV",
	"ac3":    "AC-3",
	"eac3":   "E-AC-3",
	"dts":    "DTS",
	"dca":    "DCA",
	"truehd": "TrueHD",
}

// DisplayCodecName returns a friendly label for an ffprobe codec_name value,
// falling back to the upper-cased raw name when unknown. Empty input stays empty.
// Byte-sniff labels (e.g. "H.264/AVC") are already friendly and pass through unchanged.
func DisplayCodecName(name string) string {
	if name == "" {
		return ""
	}
	if label, ok := codecDisplayNames[strings.ToLower(name)]; ok {
		return label
	}
	return strings.ToUpper(name)
}
