package config

import (
	"testing"

	"msp/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"有效端口 8080", 8080, false},
		{"有效端口 1024", 1024, false},
		{"有效端口 65535", 65535, false},
		{"无效端口 0", 0, true},
		{"无效端口 -1", -1, true},
		{"无效端口 65536", 65536, true},
		{"保留端口 80", 80, true},
		{"保留端口 443", 443, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		level   string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"error", false},
		{"none", false},
		{"", false},
		{"DEBUG", false},
		{"Info", false},
		{"invalid", true},
		{"warn", true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			err := validateLogLevel(tt.level)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePIN(t *testing.T) {
	tests := []struct {
		pin     string
		wantErr bool
	}{
		{"1234", false},
		{"12345678", false},
		{"0000", false},
		{"", true},
		{"123", true},
		{"123456789", true},
		{"12ab", true},
		{"1234 ", true},
	}

	for _, tt := range tests {
		t.Run(tt.pin, func(t *testing.T) {
			err := validatePIN(tt.pin)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"192.168.1", false},
		{"192.168.1.1.1", false},
		{"192.168.1.256", false},
		{"abc.def.ghi.jkl", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := isValidIP(tt.ip)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidIPOrCIDR(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.0/8", true},
		{"192.168.0.0/16", true},
		{"192.168.1.0/24", true},
		{"192.168.1.1/32", true},
		{"192.168.1.0/33", false},
		{"192.168.1/24", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isValidIPOrCIDR(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".mp4", true},
		{".MP4", true},
		{".tar.gz", true},
		{"mp4", false},
		{"", false},
		{".", true}, // 单个点也是有效的扩展名（虽然不太常见）
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isValidExtension(tt.ext)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		s       string
		want    int64
		wantErr bool
	}{
		{"100", 100, false},
		{"1KB", 1024, false},
		{"1MB", 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"2.5MB", 2.5 * 1024 * 1024, false},
		{"", 0, true},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got, err := parseSize(tt.s)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValidateSizeRule(t *testing.T) {
	tests := []struct {
		rule    string
		wantErr bool
	}{
		{"100MB-1GB", false},
		{">=100MB", false},
		{"<=1GB", false},
		{">100MB", false},
		{"<1GB", false},
		{"invalid", true},
		{"100MB-", true},
		{"-1GB", true},
	}

	for _, tt := range tests {
		t.Run(tt.rule, func(t *testing.T) {
			err := validateSizeRule(tt.rule)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateScope(t *testing.T) {
	tests := []struct {
		scope   string
		wantErr bool
	}{
		{"all", false},
		{"folder", false},
		{"share", false},
		{"ALL", false},
		{"Folder", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			err := validateScope(tt.scope)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("有效配置", func(t *testing.T) {
		cfg := Default()
		errors := Validate(&cfg)
		assert.Empty(t, errors)
	})

	t.Run("无效端口", func(t *testing.T) {
		cfg := Default()
		cfg.Port = 0
		errors := Validate(&cfg)
		assert.Len(t, errors, 1)
	})

	t.Run("多个错误", func(t *testing.T) {
		cfg := Default()
		cfg.Port = 0
		cfg.LogLevel = "invalid"
		cfg.Security.PINEnabled = true
		cfg.Security.PIN = "12"
		errors := Validate(&cfg)
		assert.GreaterOrEqual(t, len(errors), 3)
	})

	t.Run("nil 配置", func(t *testing.T) {
		errors := Validate(nil)
		assert.Len(t, errors, 1)
	})

	t.Run("重复共享目录", func(t *testing.T) {
		cfg := Default()
		cfg.Shares = []domain.Share{
			{Path: "/media/videos", Label: "Videos"},
			{Path: "/media/videos", Label: "Videos2"},
		}
		errors := Validate(&cfg)
		// 应该检测到重复路径
		found := false
		for _, err := range errors {
			if err.Error() != "" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestSanitize(t *testing.T) {
	t.Run("清除 PIN 码", func(t *testing.T) {
		cfg := Default()
		cfg.Security.PIN = "1234"
		cfg.Security.PINEnabled = true

		safe := Sanitize(&cfg)
		assert.Equal(t, "", safe.Security.PIN)
		assert.True(t, safe.Security.PINEnabled) // PINEnabled 应该保留
	})

	t.Run("nil 配置", func(t *testing.T) {
		safe := Sanitize(nil)
		assert.Nil(t, safe)
	})
}

func TestIsValid(t *testing.T) {
	t.Run("有效配置", func(t *testing.T) {
		cfg := Default()
		assert.True(t, IsValid(&cfg))
	})

	t.Run("无效配置", func(t *testing.T) {
		cfg := Default()
		cfg.Port = 0
		assert.False(t, IsValid(&cfg))
	})

	t.Run("nil 配置", func(t *testing.T) {
		assert.False(t, IsValid(nil))
	})
}

func TestValidateTranscodeConfig(t *testing.T) {
	t.Run("有效 auto", func(t *testing.T) {
		tc := &TranscodeConfig{HWAccel: "auto", MaxJobs: 0}
		assert.Empty(t, validateTranscodeConfig(tc))
	})

	t.Run("有效 nvenc", func(t *testing.T) {
		tc := &TranscodeConfig{HWAccel: "nvenc", MaxJobs: 4}
		assert.Empty(t, validateTranscodeConfig(tc))
	})

	t.Run("有效 none", func(t *testing.T) {
		tc := &TranscodeConfig{HWAccel: "none", MaxJobs: 2}
		assert.Empty(t, validateTranscodeConfig(tc))
	})

	t.Run("空 HWAccel 视为有效", func(t *testing.T) {
		tc := &TranscodeConfig{HWAccel: "", MaxJobs: 0}
		assert.Empty(t, validateTranscodeConfig(tc))
	})

	t.Run("无效 HWAccel", func(t *testing.T) {
		tc := &TranscodeConfig{HWAccel: "bogus", MaxJobs: 0}
		errs := validateTranscodeConfig(tc)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "hwAccel")
	})

	t.Run("负 MaxJobs", func(t *testing.T) {
		tc := &TranscodeConfig{HWAccel: "auto", MaxJobs: -1}
		errs := validateTranscodeConfig(tc)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "maxJobs")
	})

	t.Run("双重错误", func(t *testing.T) {
		tc := &TranscodeConfig{HWAccel: "invalid", MaxJobs: -5}
		errs := validateTranscodeConfig(tc)
		assert.Len(t, errs, 2)
	})
}

func TestValidate_WithEncoding(t *testing.T) {
	t.Run("默认配置含 Encoding 有效", func(t *testing.T) {
		cfg := Default()
		assert.True(t, IsValid(&cfg))
		assert.NotNil(t, cfg.Playback.Video.Encoding)
	})

	t.Run("无效 Encoding 被检出", func(t *testing.T) {
		cfg := Default()
		cfg.Playback.Video.Encoding = &TranscodeConfig{HWAccel: "xyz", MaxJobs: -1}
		errs := Validate(&cfg)
		assert.GreaterOrEqual(t, len(errs), 2)
	})
}
