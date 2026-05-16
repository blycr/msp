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
	mp := NewMediaProcessor(nil, nil)
	result := mp.DetectHWAccel(HWAccelNone)
	assert.NotNil(t, result)
	assert.False(t, result.Available, "none mode should disable hardware acceleration")
}

func TestDetectHWAccel_AutoMode(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	result := mp.DetectHWAccel(HWAccelAuto)
	assert.NotNil(t, result)
}

func TestDetectHWAccel_Idempotent(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	r1 := mp.DetectHWAccel(HWAccelNone)
	r2 := mp.DetectHWAccel(HWAccelAuto)
	assert.Equal(t, r1, r2, "DetectHWAccel must return the same pointer on repeated calls")
}

func TestGetHWAccel_BeforeDetection(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	assert.Nil(t, mp.GetHWAccel())
}

func TestDisableHWAccel(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	mp.DetectHWAccel(HWAccelNone)

	mp.hwAccel.result = &HWAccelResult{Available: true, Encoder: "h264_nvenc", Mode: HWAccelNVENC}
	assert.NotNil(t, mp.GetHWAccel())

	mp.DisableHWAccel()
	assert.Nil(t, mp.GetHWAccel(), "GetHWAccel should return nil after DisableHWAccel")
}

func TestBuildVideoArgs_Software(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	initArgs, codecArgs := mp.BuildVideoArgs("2m")
	assert.Nil(t, initArgs)
	assert.Contains(t, codecArgs, "-vcodec")
	assert.Contains(t, codecArgs, "libx264")
	assert.Contains(t, codecArgs, "-b:v")
	assert.Contains(t, codecArgs, "2m")
}

func TestBuildVideoArgs_SoftwareNoBitrate(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	initArgs, codecArgs := mp.BuildVideoArgs("")
	assert.Nil(t, initArgs)
	assert.NotContains(t, codecArgs, "-b:v")
}

func TestBuildVideoArgs_Hardware(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)
	mp.hwAccel.result = &HWAccelResult{
		Available: true,
		Encoder:   "h264_nvenc",
		InitArgs:  []string{"-hwaccel", "cuda"},
		EncArgs:   []string{"-preset", "p4"},
		Mode:      HWAccelNVENC,
	}

	initArgs, codecArgs := mp.BuildVideoArgs("4m")
	assert.Equal(t, []string{"-hwaccel", "cuda"}, initArgs)
	assert.Contains(t, codecArgs, "h264_nvenc")
	assert.Contains(t, codecArgs, "-b:v")
	assert.Contains(t, codecArgs, "4m")
}

func TestHWCandidates_NotEmpty(t *testing.T) {
	candidates := hwCandidates()
	assert.GreaterOrEqual(t, len(candidates), 2)
}

func TestFormatHWAccelStatus(t *testing.T) {
	mp := NewMediaProcessor(nil, nil)

	// Trigger path discovery so we can override the discovered value
	_ = mp.FFmpegPath()
	origPath := mp.probePaths.ffmpeg

	mp.probePaths.ffmpeg = ""
	assert.Equal(t, "unavailable (FFmpeg not found)", mp.FormatHWAccelStatus())

	mp.probePaths.ffmpeg = origPath
	if origPath == "" {
		t.Skip("FFmpeg not installed, skipping available-state tests")
	}
	assert.Equal(t, "software (libx264)", mp.FormatHWAccelStatus())

	mp.hwAccel.result = &HWAccelResult{Available: true, Encoder: "h264_nvenc", Mode: HWAccelNVENC}
	assert.Equal(t, "hardware (h264_nvenc)", mp.FormatHWAccelStatus())
}
