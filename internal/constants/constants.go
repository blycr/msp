// Package constants 定义了应用程序中使用的所有常量。
// 集中管理常量有助于维护和避免魔法数字。
package constants

import "time"

// 网络与服务器常量
const (
	// DefaultPort 是 HTTP 服务的默认端口
	DefaultPort = 8099

	// DefaultPIN 是默认的 PIN 码
	DefaultPIN = "0000"
)

// 文件系统权限
const (
	// FilePerm 是常规文件的权限 (rw-------)
	FilePerm = 0600

	// DirPerm 是目录的权限 (rwxr-x---)
	DirPerm = 0750
)

// 日志相关常量
const (
	// LogRotateSize 是日志文件轮转的大小阈值 (10MB)
	LogRotateSize = 10 * 1024 * 1024

	// LogRotateCheckInterval 是检查日志轮转的日志条数间隔
	LogRotateCheckInterval = 100

	// LogTimeFormat 是日志时间格式
	LogTimeFormat = "2006/01/02 15:04:05.000000"
)

// 缓存与定时器
const (
	// MediaCacheTTL 是媒体缓存的有效期
	MediaCacheTTL = 2 * time.Minute

	// ConfigCheckInterval 是配置文件检查间隔
	ConfigCheckInterval = 2 * time.Second

	// CookieMaxAge 是 PIN cookie 的最大存活时间 (7天)
	CookieMaxAge = 86400 * 7
)

// 扫描限制
const (
	// DefaultScanLimit 是默认的媒体扫描限制数
	DefaultScanLimit = 100000

	// DBScanLimit 是数据库扫描的默认限制数 (近似无限)
	DBScanLimit = 1000000000
)

// 私有网络 IP 段
const (
	// PrivateClassA 是 A 类私有网络起始地址
	PrivateClassA = 10

	// PrivateClassBStart 是 B 类私有网络起始地址
	PrivateClassBStart = 172

	// PrivateClassBMin 是 B 类私有网络最小子网
	PrivateClassBMin = 16

	// PrivateClassBMax 是 B 类私有网络最大子网
	PrivateClassBMax = 31

	// PrivateClassC1 是 C 类私有网络第一段
	PrivateClassC1 = 192

	// PrivateClassC2 是 C 类私有网络第二段
	PrivateClassC2 = 168
)

// 字节单位
const (
	// BytesPerKB 是每 KB 的字节数
	BytesPerKB = 1024

	// BytesPerMB 是每 MB 的字节数
	BytesPerMB = 1024 * 1024

	// BytesPerGB 是每 GB 的字节数
	BytesPerGB = 1024 * 1024 * 1024

	// BytesPerTB 是每 TB 的字节数
	BytesPerTB = 1024 * 1024 * 1024 * 1024
)

// HTTP 状态码 (常用)
const (
	StatusOK                  = 200
	StatusBadRequest          = 400
	StatusUnauthorized        = 401
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusMethodNotAllowed    = 405
	StatusInternalServerError = 500
	StatusNotModified         = 304
	StatusNoContent           = 204
)

// 网络地址
const (
	LocalhostIPv4 = "127.0.0.1"
	LocalhostIPv6 = "::1"
)
