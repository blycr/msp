package media

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidHWAccelModes(t *testing.T) {
	valid := []HWAccelMode{"auto", "nvenc", "qsv", "amf", "vaapi", "videotoolbox", "none"}
	for _, m := range valid {
		assert.True(t, ValidHWAccelModes[m], "expected %q to be valid", m)
	}
	assert.False(t, ValidHWAccelModes["bogus"])
	assert.False(t, ValidHWAccelModes[""])
}

func TestDetectHWAccel_NoneMode(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	result := DetectHWAccel(HWAccelNone)
	assert.NotNil(t, result)
	assert.False(t, result.Available, "none mode should disable hardware acceleration")
}

func TestDetectHWAccel_AutoMode(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	// auto mode should not panic regardless of environment
	result := DetectHWAccel(HWAccelAuto)
	assert.NotNil(t, result)
	// result.Available depends on actual hardware — we only verify no crash
}

func TestDetectHWAccel_Idempotent(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	r1 := DetectHWAccel(HWAccelNone)
	r2 := DetectHWAccel(HWAccelAuto) // second call should return cached result
	assert.Equal(t, r1, r2, "DetectHWAccel must return the same pointer on repeated calls")
}

func TestGetHWAccel_BeforeDetection(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	// Before detection, should return nil
	assert.Nil(t, GetHWAccel())
}

func TestDisableHWAccel(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	// Detect with none first to get a non-nil result
	DetectHWAccel(HWAccelNone)

	// Force a fake available result for testing
	hwResult = &HWAccelResult{Available: true, Encoder: "h264_nvenc", Mode: HWAccelNVENC}
	assert.NotNil(t, GetHWAccel())

	DisableHWAccel()
	assert.Nil(t, GetHWAccel(), "GetHWAccel should return nil after DisableHWAccel")
}

func TestBuildVideoArgs_Software(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	// No detection done → software path
	initArgs, codecArgs := BuildVideoArgs("2m")
	assert.Nil(t, initArgs)
	assert.Contains(t, codecArgs, "-vcodec")
	assert.Contains(t, codecArgs, "libx264")
	assert.Contains(t, codecArgs, "-b:v")
	assert.Contains(t, codecArgs, "2m")
}

func TestBuildVideoArgs_SoftwareNoBitrate(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	initArgs, codecArgs := BuildVideoArgs("")
	assert.Nil(t, initArgs)
	assert.NotContains(t, codecArgs, "-b:v")
}

func TestBuildVideoArgs_Hardware(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	// Inject a fake hardware result
	hwResult = &HWAccelResult{
		Available: true,
		Encoder:   "h264_nvenc",
		InitArgs:  []string{"-hwaccel", "cuda"},
		EncArgs:   []string{"-preset", "p4"},
		Mode:      HWAccelNVENC,
	}

	initArgs, codecArgs := BuildVideoArgs("4m")
	assert.Equal(t, []string{"-hwaccel", "cuda"}, initArgs)
	assert.Contains(t, codecArgs, "h264_nvenc")
	assert.Contains(t, codecArgs, "-b:v")
	assert.Contains(t, codecArgs, "4m")
}

func TestHWCandidates_NotEmpty(t *testing.T) {
	candidates := hwCandidates()
	// At least NVENC and QSV should be present on all platforms
	assert.GreaterOrEqual(t, len(candidates), 2)
}

func TestFormatHWAccelStatus(t *testing.T) {
	ResetHWAccelForTest()
	defer ResetHWAccelForTest()

	// Trigger path discovery first so pathOnce fires, then we can override
	_ = FFmpegPath()
	origPath := ffmpegPath
	defer func() { ffmpegPath = origPath }()

	// When FFmpeg is not found, status should reflect that
	ffmpegPath = ""
	assert.Equal(t, "unavailable (FFmpeg not found)", FormatHWAccelStatus())

	// Restore real path for remaining assertions (skip if ffmpeg not installed)
	if origPath == "" {
		t.Skip("FFmpeg not installed, skipping available-state tests")
	}
	ffmpegPath = origPath
	assert.Equal(t, "software (libx264)", FormatHWAccelStatus())

	hwResult = &HWAccelResult{Available: true, Encoder: "h264_nvenc", Mode: HWAccelNVENC}
	assert.Equal(t, "hardware (h264_nvenc)", FormatHWAccelStatus())
}
