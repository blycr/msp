package media

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// HWAccelMode represents the user's hardware acceleration preference.
type HWAccelMode string

const (
	HWAccelAuto         HWAccelMode = "auto"
	HWAccelNVENC        HWAccelMode = "nvenc"
	HWAccelQSV          HWAccelMode = "qsv"
	HWAccelAMF          HWAccelMode = "amf"
	HWAccelVAAPI        HWAccelMode = "vaapi"
	HWAccelVideoToolbox HWAccelMode = "videotoolbox"
	HWAccelNone         HWAccelMode = "none"
)

// ValidHWAccelModes is the whitelist of accepted configuration values.
var ValidHWAccelModes = map[HWAccelMode]bool{
	HWAccelAuto:         true,
	HWAccelNVENC:        true,
	HWAccelQSV:          true,
	HWAccelAMF:          true,
	HWAccelVAAPI:        true,
	HWAccelVideoToolbox: true,
	HWAccelNone:         true,
}

// hwEncoder describes one candidate hardware encoder.
type hwEncoder struct {
	mode     HWAccelMode
	encoder  string
	initArgs []string
	encArgs  []string
}

// hwCandidates returns platform-filtered candidates in priority order.
func hwCandidates() []hwEncoder {
	os := runtime.GOOS
	all := []hwEncoder{
		{
			mode:     HWAccelNVENC,
			encoder:  "h264_nvenc",
			initArgs: []string{"-hwaccel", "cuda"},
			encArgs:  []string{"-preset", "p4", "-tune", "ll", "-pix_fmt", "yuv420p"},
		},
		{
			mode:     HWAccelQSV,
			encoder:  "h264_qsv",
			initArgs: []string{"-hwaccel", "qsv"},
			encArgs:  []string{"-preset", "fast", "-global_quality", "23"},
		},
		{
			mode:    HWAccelAMF,
			encoder: "h264_amf",
			encArgs: []string{"-quality", "speed", "-pix_fmt", "yuv420p"},
		},
		{
			mode:     HWAccelVAAPI,
			encoder:  "h264_vaapi",
			initArgs: []string{"-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD128", "-hwaccel_output_format", "vaapi"},
			encArgs:  nil,
		},
		{
			mode:    HWAccelVideoToolbox,
			encoder: "h264_videotoolbox",
			encArgs: []string{"-allow_sw", "1", "-pix_fmt", "yuv420p"},
		},
	}

	var filtered []hwEncoder
	for _, c := range all {
		switch c.mode {
		case HWAccelVAAPI:
			if os == "linux" {
				filtered = append(filtered, c)
			}
		case HWAccelVideoToolbox:
			if os == "darwin" {
				filtered = append(filtered, c)
			}
		case HWAccelAMF:
			if os == "windows" {
				filtered = append(filtered, c)
			}
		default:
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// HWAccelResult holds the detected (or disabled) hardware acceleration state.
type HWAccelResult struct {
	Available bool
	Encoder   string
	InitArgs  []string
	EncArgs   []string
	Mode      HWAccelMode
}

// DetectHWAccel probes FFmpeg for a usable hardware encoder.
func (mp *MediaProcessor) DetectHWAccel(mode HWAccelMode) *HWAccelResult {
	mp.hwAccel.once.Do(func() {
		mp.hwAccel.result = mp.detectHWAccelOnce(mode)
	})
	return mp.hwAccel.result
}

// GetHWAccel returns the cached detection result, or nil if disabled at runtime.
func (mp *MediaProcessor) GetHWAccel() *HWAccelResult {
	if mp.hwAccel.disabled.Load() {
		return nil
	}
	return mp.hwAccel.result
}

// DisableHWAccel marks hardware acceleration as failed at runtime.
func (mp *MediaProcessor) DisableHWAccel() {
	mp.hwAccel.disabled.Store(true)
	slog.Warn("hardware acceleration disabled due to runtime failure, falling back to software encoding")
}

func (mp *MediaProcessor) detectHWAccelOnce(mode HWAccelMode) *HWAccelResult {
	none := &HWAccelResult{Available: false}

	if mode == HWAccelNone {
		slog.Info("hardware acceleration disabled by configuration")
		return none
	}

	if !mp.CheckFFmpeg() {
		return none
	}

	candidates := hwCandidates()

	if mode != HWAccelAuto {
		var specific []hwEncoder
		for _, c := range candidates {
			if c.mode == mode {
				specific = append(specific, c)
			}
		}
		if len(specific) == 0 {
			slog.Warn("requested hardware encoder not available", "mode", mode, "os", runtime.GOOS)
			return none
		}
		candidates = specific
	}

	for _, c := range candidates {
		if mp.probeEncoder(c) {
			slog.Info("hardware acceleration enabled", "encoder", c.encoder, "mode", c.mode)
			return &HWAccelResult{
				Available: true,
				Encoder:   c.encoder,
				InitArgs:  c.initArgs,
				EncArgs:   c.encArgs,
				Mode:      c.mode,
			}
		}
	}

	slog.Info("no hardware encoder available, using software encoding")
	return none
}

func (mp *MediaProcessor) probeEncoder(enc hwEncoder) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"-hide_banner", "-loglevel", "error"}
	args = append(args, enc.initArgs...)
	args = append(args, "-f", "lavfi", "-i", "nullsrc=s=256x256:d=0.1:r=1")
	args = append(args, "-c:v", enc.encoder)
	args = append(args, enc.encArgs...)
	args = append(args, "-frames:v", "1", "-f", "null", "-")

	ffmpegBin := mp.FFmpegPath()
	if ffmpegBin == "" {
		return false
	}

	//nolint:gosec // args are from internal whitelist, not user input
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Debug("encoder probe failed", "encoder", enc.encoder, "err", err, "stderr", strings.TrimSpace(stderr.String()))
		return false
	}
	return true
}

// BuildVideoArgs returns the FFmpeg arguments for video encoding.
func (mp *MediaProcessor) BuildVideoArgs(bitrate string) (initArgs []string, codecArgs []string) {
	hw := mp.GetHWAccel()
	if hw != nil && hw.Available {
		initArgs = make([]string, len(hw.InitArgs))
		copy(initArgs, hw.InitArgs)

		codecArgs = []string{"-c:v", hw.Encoder}
		codecArgs = append(codecArgs, hw.EncArgs...)
		if bitrate != "" {
			codecArgs = append(codecArgs, "-b:v", bitrate)
		}
		return initArgs, codecArgs
	}

	// Software fallback
	codecArgs = []string{"-vcodec", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p", "-threads", "2"}
	if bitrate != "" {
		codecArgs = append(codecArgs, "-b:v", bitrate)
	}
	return nil, codecArgs
}

// FormatHWAccelStatus returns a human-readable status string for logging.
func (mp *MediaProcessor) FormatHWAccelStatus() string {
	if !mp.FFmpegAvailable() {
		return "unavailable (FFmpeg not found)"
	}
	r := mp.GetHWAccel()
	if r == nil || !r.Available {
		return "software (libx264)"
	}
	return fmt.Sprintf("hardware (%s)", r.Encoder)
}
