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
)

func EncodeID(absPath string) string {
	b := []byte(absPath)
	return base64.RawURLEncoding.EncodeToString(b)
}

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

func NormalizeWinPath(p string) string { return NormalizePath(p) }

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

func ParseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	scale := int64(1)
	if strings.HasSuffix(s, "TB") {
		scale = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "GB") {
		scale = 1024 * 1024 * 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "MB") {
		scale = 1024 * 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "KB") {
		scale = 1024
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "B") {
		s = s[:len(s)-1]
	}

	val, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return int64(val * float64(scale))
}

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

func Itoa(i int) string {
	return strconv.Itoa(i)
}

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

func MustExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

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

func IsExistingDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func IsAllowedFile(fileAbs string, shares []config.Share) bool {
	if fileAbs == "" {
		return false
	}
	f, err := filepath.Abs(fileAbs)
	if err != nil {
		return false
	}
	f = filepath.Clean(f)

	for _, sh := range shares {
		root := NormalizePath(sh.Path)
		if root == "" {
			continue
		}
		if WithinRoot(root, f) {
			st, err := os.Stat(f)
			return err == nil && !st.IsDir()
		}
	}
	return false
}

func WithinWinRoot(root, target string) bool { return WithinRoot(root, target) }

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

func SamePathWin(a, b string) bool { return SamePath(a, b) }

func SamePath(a, b string) bool {
	na := NormalizePath(a)
	nb := NormalizePath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(na, nb)
	}
	return na == nb
}

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

func IsPrivateIPv4(ip net.IP) bool {
	if ip == nil || len(ip) != 4 {
		return false
	}
	switch {
	case ip[0] == 10:
		return true
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return true
	case ip[0] == 192 && ip[1] == 168:
		return true
	default:
		return false
	}
}

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
