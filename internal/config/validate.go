package config

import (
	"fmt"
	"msp/internal/domain"
	"net"
	"strconv"
	"strings"
)

// ValidationError 表示配置验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("配置验证错误 [%s]: %s", e.Field, e.Message)
}

// Validate 验证配置是否有效
func Validate(cfg *Config) []error {
	if cfg == nil {
		return []error{&ValidationError{Field: "config", Message: "配置不能为空"}}
	}

	var errors []error

	// 验证端口
	if err := validatePort(cfg.Port); err != nil {
		errors = append(errors, err)
	}

	// 验证日志级别
	if err := validateLogLevel(cfg.LogLevel); err != nil {
		errors = append(errors, err)
	}

	// 验证共享目录
	if errs := validateShares(cfg.Shares); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// 验证安全配置
	if errs := validateSecurity(&cfg.Security); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// 验证黑名单配置
	if errs := validateBlacklist(&cfg.Blacklist); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// 验证播放配置
	if errs := validatePlayback(&cfg.Playback); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	return errors
}

// validatePort 验证端口号
func validatePort(port int) error {
	if port <= 0 {
		return &ValidationError{Field: "port", Message: "端口必须大于 0"}
	}
	if port > 65535 {
		return &ValidationError{Field: "port", Message: "端口必须在 1-65535 范围内"}
	}
	// 保留端口检查
	if port < 1024 {
		return &ValidationError{Field: "port", Message: "端口小于 1024 需要管理员权限"}
	}
	return nil
}

// validateLogLevel 验证日志级别
func validateLogLevel(level string) error {
	validLevels := []string{"debug", "info", "error", "none", ""}
	level = strings.ToLower(strings.TrimSpace(level))
	for _, valid := range validLevels {
		if level == valid {
			return nil
		}
	}
	return &ValidationError{
		Field:   "logLevel",
		Message: fmt.Sprintf("无效的日志级别: %s，必须是 debug、info、error 或 none", level),
	}
}

// validateShares 验证共享目录配置
func validateShares(shares []domain.Share) []error {
	var errors []error
	seenPaths := make(map[string]bool)
	seenLabels := make(map[string]bool)

	for i, share := range shares {
		prefix := fmt.Sprintf("shares[%d]", i)

		// 验证路径
		if strings.TrimSpace(share.Path) == "" {
			errors = append(errors, &ValidationError{
				Field:   prefix + ".path",
				Message: "共享目录路径不能为空",
			})
			continue
		}

		// 检查重复路径
		pathKey := strings.ToLower(strings.TrimSpace(share.Path))
		if seenPaths[pathKey] {
			errors = append(errors, &ValidationError{
				Field:   prefix + ".path",
				Message: fmt.Sprintf("重复的共享目录路径: %s", share.Path),
			})
		}
		seenPaths[pathKey] = true

		// 验证标签
		label := strings.TrimSpace(share.Label)
		if label == "" {
			errors = append(errors, &ValidationError{
				Field:   prefix + ".label",
				Message: "共享目录标签不能为空",
			})
		} else {
			// 检查重复标签
			labelKey := strings.ToLower(label)
			if seenLabels[labelKey] {
				errors = append(errors, &ValidationError{
					Field:   prefix + ".label",
					Message: fmt.Sprintf("重复的共享目录标签: %s", label),
				})
			}
			seenLabels[labelKey] = true
		}
	}

	return errors
}

// validateSecurity 验证安全配置
func validateSecurity(sec *SecurityConfig) []error {
	var errors []error

	// 验证 PIN 码
	if sec.PINEnabled {
		if err := validatePIN(sec.PIN); err != nil {
			errors = append(errors, &ValidationError{
				Field:   "security.pin",
				Message: err.Error(),
			})
		}
	}

	// 验证 IP 白名单
	for i, ip := range sec.IPWhitelist {
		if !isValidIPOrCIDR(ip) {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("security.ipWhitelist[%d]", i),
				Message: fmt.Sprintf("无效的 IP 地址或 CIDR: %s", ip),
			})
		}
	}

	// 验证 IP 黑名单
	for i, ip := range sec.IPBlacklist {
		if !isValidIPOrCIDR(ip) {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("security.ipBlacklist[%d]", i),
				Message: fmt.Sprintf("无效的 IP 地址或 CIDR: %s", ip),
			})
		}
	}

	return errors
}

// validatePIN 验证 PIN 码格式
func validatePIN(pin string) error {
	if pin == "" {
		return fmt.Errorf("PIN 码不能为空")
	}
	if len(pin) < 4 {
		return fmt.Errorf("PIN 码长度至少为 4 位")
	}
	if len(pin) > 8 {
		return fmt.Errorf("PIN 码长度不能超过 8 位")
	}
	// 只允许数字
	for _, c := range pin {
		if c < '0' || c > '9' {
			return fmt.Errorf("PIN 码只能包含数字")
		}
	}
	return nil
}

// isValidIPOrCIDR 验证 IP 地址或 CIDR 格式
func isValidIPOrCIDR(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// 检查 CIDR 格式
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		return err == nil
	}

	// 验证 IP 地址
	return isValidIP(s)
}

// isValidIP 验证 IP 地址（IPv4 或 IPv6）
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}

// validateBlacklist 验证黑名单配置
func validateBlacklist(bl *BlacklistConfig) []error {
	var errors []error

	// 验证扩展名格式
	for i, ext := range bl.Extensions {
		if !isValidExtension(ext) {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("blacklist.extensions[%d]", i),
				Message: fmt.Sprintf("无效的扩展名格式: %s（应以 . 开头）", ext),
			})
		}
	}

	// 验证文件大小规则
	if bl.SizeRule != "" {
		if err := validateSizeRule(bl.SizeRule); err != nil {
			errors = append(errors, &ValidationError{
				Field:   "blacklist.sizeRule",
				Message: err.Error(),
			})
		}
	}

	return errors
}

// isValidExtension 验证扩展名格式
func isValidExtension(ext string) bool {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return false
	}
	// 扩展名应该以 . 开头
	return strings.HasPrefix(ext, ".")
}

// validateSizeRule 验证文件大小规则
func validateSizeRule(rule string) error {
	rule = strings.TrimSpace(strings.ToUpper(rule))

	// 范围格式: min-max
	if strings.Contains(rule, "-") {
		parts := strings.Split(rule, "-")
		if len(parts) != 2 {
			return fmt.Errorf("无效的大小范围格式")
		}
		if _, err := parseSize(parts[0]); err != nil {
			return fmt.Errorf("无效的最小值: %v", err)
		}
		if _, err := parseSize(parts[1]); err != nil {
			return fmt.Errorf("无效的最大值: %v", err)
		}
		return nil
	}

	// 比较格式: >=, <=, >, <
	for _, prefix := range []string{">=", "<=", ">", "<"} {
		if strings.HasPrefix(rule, prefix) {
			val := strings.TrimPrefix(rule, prefix)
			if _, err := parseSize(val); err != nil {
				return fmt.Errorf("无效的大小值: %v", err)
			}
			return nil
		}
	}

	return fmt.Errorf("无效的大小规则格式")
}

// parseSize 解析带单位的大小
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("空值")
	}

	multiplier := int64(1)
	if strings.HasSuffix(s, "TB") {
		multiplier = 1 << 40
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "GB") {
		multiplier = 1 << 30
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1 << 20
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1 << 10
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "B") {
		s = s[:len(s)-1]
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}

	return int64(val * float64(multiplier)), nil
}

// validatePlayback 验证播放配置
func validatePlayback(pb *PlaybackConfig) []error {
	var errors []error

	// 验证音频配置
	if pb.Audio.Scope != nil {
		if err := validateScope(*pb.Audio.Scope); err != nil {
			errors = append(errors, &ValidationError{
				Field:   "playback.audio.scope",
				Message: err.Error(),
			})
		}
	}

	// 验证视频配置
	if pb.Video.Scope != nil {
		if err := validateScope(*pb.Video.Scope); err != nil {
			errors = append(errors, &ValidationError{
				Field:   "playback.video.scope",
				Message: err.Error(),
			})
		}
	}

	// 验证转码编码配置
	if pb.Video.Encoding != nil {
		if errs := validateTranscodeConfig(pb.Video.Encoding); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	// 验证图片配置
	if pb.Image.Scope != nil {
		if err := validateScope(*pb.Image.Scope); err != nil {
			errors = append(errors, &ValidationError{
				Field:   "playback.image.scope",
				Message: err.Error(),
			})
		}
	}

	return errors
}

// validHWAccelValues is the whitelist of accepted hardware acceleration modes.
var validHWAccelValues = map[string]bool{
	"auto": true, "nvenc": true, "qsv": true, "amf": true,
	"vaapi": true, "videotoolbox": true, "none": true, "": true,
}

// validateTranscodeConfig 验证转码编码配置
func validateTranscodeConfig(tc *TranscodeConfig) []error {
	var errors []error

	hw := strings.ToLower(strings.TrimSpace(tc.HWAccel))
	if !validHWAccelValues[hw] {
		errors = append(errors, &ValidationError{
			Field:   "playback.video.encoding.hwAccel",
			Message: fmt.Sprintf("无效的硬件加速模式: %s，可选值: auto, nvenc, qsv, amf, vaapi, videotoolbox, none", tc.HWAccel),
		})
	}

	if tc.MaxJobs < 0 {
		errors = append(errors, &ValidationError{
			Field:   "playback.video.encoding.maxJobs",
			Message: fmt.Sprintf("并发数不能为负数: %d", tc.MaxJobs),
		})
	}

	return errors
}

// validateScope 验证播放范围
func validateScope(scope string) error {
	validScopes := []string{"all", "folder", "share"}
	scope = strings.ToLower(strings.TrimSpace(scope))
	for _, valid := range validScopes {
		if scope == valid {
			return nil
		}
	}
	return fmt.Errorf("无效的播放范围: %s，必须是 all、folder 或 share", scope)
}

// Sanitize 清理配置中的敏感信息，返回一个安全的副本
func Sanitize(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	// 创建副本
	safe := *cfg

	// 清除 PIN 码
	safe.Security.PIN = ""

	return &safe
}

// IsValid 快速检查配置是否有效
func IsValid(cfg *Config) bool {
	return len(Validate(cfg)) == 0
}
