package media

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
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
	encoder  string   // FFmpeg encoder name, e.g. "h264_nvenc"
	initArgs []string // args placed before -i
	encArgs  []string // args placed after encoder selection
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

	// Platform filter
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
			// NVENC and QSV work on all desktop platforms
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// HWAccelResult holds the detected (or disabled) hardware acceleration state.
type HWAccelResult struct {
	Available bool
	Encoder   string   // e.g. "h264_nvenc"
	InitArgs  []string // e.g. ["-hwaccel", "cuda"]
	EncArgs   []string // encoder-specific args
	Mode      HWAccelMode
}

var (
	hwOnce   sync.Once
	hwResult *HWAccelResult

	// hwDisabled is set atomically when a runtime failure triggers fallback.
	hwMu       sync.RWMutex
	hwDisabled bool
)

// DetectHWAccel probes FFmpeg for a usable hardware encoder.
// It is safe to call from multiple goroutines; detection runs at most once.
// The mode parameter comes from user configuration ("auto", "nvenc", "none", etc.).
func DetectHWAccel(mode HWAccelMode) *HWAccelResult {
	hwOnce.Do(func() {
		hwResult = detectHWAccelOnce(mode)
	})
	return hwResult
}

// GetHWAccel returns the cached detection result, or nil if not yet probed
// or if hardware acceleration was disabled at runtime.
func GetHWAccel() *HWAccelResult {
	hwMu.RLock()
	disabled := hwDisabled
	hwMu.RUnlock()
	if disabled {
		return nil
	}
	return hwResult
}

// DisableHWAccel marks hardware acceleration as failed at runtime.
// Subsequent calls to GetHWAccel will return nil.
func DisableHWAccel() {
	hwMu.Lock()
	hwDisabled = true
	hwMu.Unlock()
	log.Printf("[WARN] Hardware acceleration disabled due to runtime failure, falling back to software encoding")
}

func detectHWAccelOnce(mode HWAccelMode) *HWAccelResult {
	none := &HWAccelResult{Available: false}

	if mode == HWAccelNone {
		log.Printf("[INFO] Hardware acceleration disabled by configuration")
		return none
	}

	if !CheckFFmpeg() {
		return none
	}

	candidates := hwCandidates()

	// If user specified a particular encoder, filter to that one.
	if mode != HWAccelAuto {
		var specific []hwEncoder
		for _, c := range candidates {
			if c.mode == mode {
				specific = append(specific, c)
			}
		}
		if len(specific) == 0 {
			log.Printf("[WARN] Requested hardware encoder %q not available on %s", mode, runtime.GOOS)
			return none
		}
		candidates = specific
	}

	// Probe each candidate with a real encode test.
	for _, c := range candidates {
		if probeEncoder(c) {
			log.Printf("[INFO] Hardware acceleration enabled: %s (%s)", c.encoder, c.mode)
			return &HWAccelResult{
				Available: true,
				Encoder:   c.encoder,
				InitArgs:  c.initArgs,
				EncArgs:   c.encArgs,
				Mode:      c.mode,
			}
		}
	}

	log.Printf("[INFO] No hardware encoder available, using software encoding")
	return none
}

// probeEncoder runs a quick null-encode to verify the encoder actually works
// (i.e. driver is installed and GPU is accessible).
func probeEncoder(enc hwEncoder) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"-hide_banner", "-loglevel", "error"}
	args = append(args, enc.initArgs...)
	args = append(args, "-f", "lavfi", "-i", "nullsrc=s=256x256:d=0.1:r=1")
	args = append(args, "-c:v", enc.encoder)
	args = append(args, enc.encArgs...)
	args = append(args, "-frames:v", "1", "-f", "null", "-")

	//nolint:gosec // args are from internal whitelist, not user input
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("[DEBUG] Probe %s failed: %v (stderr: %s)",
			enc.encoder, err, strings.TrimSpace(stderr.String()))
		return false
	}
	return true
}

// BuildVideoArgs returns the FFmpeg arguments for video encoding.
// This is the single integration point used by TranscodeStream.
// If hardware acceleration is available, it returns hw-accelerated args;
// otherwise it returns the original software args.
func BuildVideoArgs(bitrate string) (initArgs []string, codecArgs []string) {
	hw := GetHWAccel()
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

	// Software fallback — identical to original behaviour
	codecArgs = []string{"-vcodec", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p", "-threads", "2"}
	if bitrate != "" {
		codecArgs = append(codecArgs, "-b:v", bitrate)
	}
	return nil, codecArgs
}

// ResetHWAccelForTest is exported only for unit-test teardown.
func ResetHWAccelForTest() {
	hwOnce = sync.Once{}
	hwResult = nil
	hwMu.Lock()
	hwDisabled = false
	hwMu.Unlock()
}

// FormatHWAccelStatus returns a human-readable status string for logging.
func FormatHWAccelStatus() string {
	r := GetHWAccel()
	if r == nil || !r.Available {
		return "software (libx264)"
	}
	return fmt.Sprintf("hardware (%s)", r.Encoder)
}
