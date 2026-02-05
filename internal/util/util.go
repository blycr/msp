package util

import (
	"encoding/base64"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"msp/internal/config"
	"msp/internal/constants"
)

// EncodeID 将绝对路径编码为 Base64 URL 安全的字符串。
func EncodeID(absPath string) string {
	b := []byte(absPath)
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeID 将 Base64 编码的 ID 解码回原始路径。
func DecodeID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("empty")
	}
	return string(b), nil
}

// NormalizePath 规范化路径，去除引号，转换为系统路径分隔符，并返回绝对路径。
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, `"`, "")
	p = filepath.FromSlash(p)
	p = filepath.Clean(p)
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	return p
}

// ParseSize 解析带单位的大小字符串（如 "1GB", "100MB"）为字节数。
func ParseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	scale := int64(1)
	if strings.HasSuffix(s, "TB") {
		scale = constants.BytesPerTB
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "GB") {
		scale = constants.BytesPerGB
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "MB") {
		scale = constants.BytesPerMB
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "KB") {
		scale = constants.BytesPerKB
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "B") {
		s = s[:len(s)-1]
	}

	val, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return int64(val * float64(scale))
}

// U64Base36 将 uint64 转换为 36 进制字符串（0-9a-z）。
func U64Base36(u uint64) string {
	if u == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var b [32]byte
	pos := len(b)
	for u > 0 {
		pos--
		b[pos] = digits[u%36]
		u /= 36
	}
	return string(b[pos:])
}

// Itoa 将整数转换为字符串。
func Itoa(i int) string {
	return strconv.Itoa(i)
}

// DedupeShares 去除共享目录列表中的重复项（不区分大小写）。
func DedupeShares(in []config.Share) []config.Share {
	out := make([]config.Share, 0, len(in))
	seen := map[string]bool{}
	for _, sh := range in {
		key := strings.ToLower(sh.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, sh)
	}
	return out
}

// MustExeDir 返回可执行文件所在目录，如果失败则返回 "."。
func MustExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// NormalizeShares 规范化共享目录列表，清理路径并为空标签生成默认标签。
func NormalizeShares(in []config.Share) []config.Share {
	out := make([]config.Share, 0, len(in))
	for _, sh := range in {
		p := NormalizePath(sh.Path)
		if p == "" {
			continue
		}
		lbl := strings.TrimSpace(sh.Label)
		if lbl == "" {
			lbl = filepath.Base(p)
		}
		out = append(out, config.Share{Label: lbl, Path: p})
	}
	return out
}

// IsExistingDir 检查路径是否存在且是目录。
func IsExistingDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// IsAllowedFile 检查文件是否在允许的共享目录内且存在。
// 安全特性：解析符号链接，防止通过符号链接绕过目录限制。
func IsAllowedFile(fileAbs string, shares []config.Share) bool {
	if fileAbs == "" {
		return false
	}
	
	// 获取绝对路径
	f, err := filepath.Abs(fileAbs)
	if err != nil {
		return false
	}
	f = filepath.Clean(f)
	
	// 解析符号链接获取真实路径（安全关键）
	realPath, err := filepath.EvalSymlinks(f)
	if err != nil {
		// 如果无法解析符号链接，使用原始路径
		realPath = f
	}
	realPath = filepath.Clean(realPath)

	for _, sh := range shares {
		root := NormalizePath(sh.Path)
		if root == "" {
			continue
		}
		// 检查真实路径是否在允许目录内
		if WithinRoot(root, realPath) {
			st, err := os.Stat(realPath)
			return err == nil && !st.IsDir()
		}
	}
	return false
}

// WithinRoot 检查目标路径是否在根目录内（防止目录遍历）。
func WithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if runtime.GOOS == "windows" {
		if strings.EqualFold(root, target) {
			return true
		}
		rs := root
		if !strings.HasSuffix(rs, string(os.PathSeparator)) {
			rs += string(os.PathSeparator)
		}
		return strings.HasPrefix(strings.ToLower(target), strings.ToLower(rs))
	}
	if root == target {
		return true
	}
	rs := root
	if !strings.HasSuffix(rs, string(os.PathSeparator)) {
		rs += string(os.PathSeparator)
	}
	return strings.HasPrefix(target, rs)
}

// SamePath 检查两个路径是否指向同一位置（考虑 Windows 大小写不敏感）。
func SamePath(a, b string) bool {
	na := NormalizePath(a)
	nb := NormalizePath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(na, nb)
	}
	return na == nb
}

// GetLanIPv4s 获取所有本地私有网络的 IPv4 地址列表。
func GetLanIPv4s() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}
			if IsPrivateIPv4(ip4) {
				ips = append(ips, ip4.String())
			}
		}
	}
	sort.Strings(ips)
	ips = DedupeStrings(ips)
	return ips
}

// IsPrivateIPv4 检查 IP 是否是 RFC1918 私有网络地址。
func IsPrivateIPv4(ip net.IP) bool {
	if ip == nil || len(ip) != 4 {
		return false
	}
	switch {
	case ip[0] == constants.PrivateClassA:
		return true
	case ip[0] == constants.PrivateClassBStart && ip[1] >= constants.PrivateClassBMin && ip[1] <= constants.PrivateClassBMax:
		return true
	case ip[0] == constants.PrivateClassC1 && ip[1] == constants.PrivateClassC2:
		return true
	default:
		return false
	}
}

// DedupeStrings 去除字符串切片中的重复项。
func DedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
