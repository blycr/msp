# MSP (Media Share & Preview) - CodeMap

> 项目架构全景图与代码导航指南

---

## 1. 项目概述

MSP 是一个基于 Go + 原生 JavaScript 的媒体共享与预览服务，支持视频、音频、图片的在线播放与转码。

### 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端 | Go 1.24 | 纯 Go 实现，无 CGO 依赖 |
| 数据库 | SQLite (modernc.org/sqlite) | 纯 Go SQLite 驱动 |
| 前端 | Vite 8 + Vanilla JS | 无框架依赖 |
| 转码 | FFmpeg | 支持硬件加速 (NVENC/QSV/AMF/VAAPI) |
| 包管理 | bun 1.3 | 前端依赖管理 |

---

## 2. 目录结构

```
msp/
├── cmd/msp/                    # 应用程序入口
│   └── main.go                 # 主程序：服务启动、路由注册、优雅关闭
├── internal/                   # 内部包（私有）
│   ├── cache/                  # 缓存层
│   │   └── media.go            # 媒体缓存管理（内存+磁盘）
│   ├── config/                 # 配置管理
│   │   ├── config.go           # 配置结构定义与默认值
│   │   └── validate.go         # 配置验证逻辑
│   ├── constants/              # 常量定义
│   │   ├── constants.go        # 通用常量
│   │   └── errors.go           # 错误消息常量
│   ├── domain/                 # 领域模型（核心数据结构）
│   │   ├── types.go            # 实体定义：MediaItem, PlaybackProgress 等
│   │   └── share.go            # 共享目录相关定义
│   ├── handler/                # HTTP 处理器层
│   │   ├── handler.go          # Handler 结构体与依赖注入
│   │   ├── media.go            # /api/media 接口
│   │   ├── stream.go           # /api/stream 流媒体传输
│   │   ├── config.go           # /api/config 配置接口
│   │   ├── progress.go         # /api/progress 播放进度
│   │   ├── subtitle.go         # /api/subtitle 字幕处理
│   │   ├── auth.go             # /api/pin 认证接口
│   │   ├── middleware.go       # HTTP 中间件（Gzip、安全头、日志）
│   │   └── common.go           # 通用响应工具
│   ├── media/                  # 媒体处理核心
│   │   ├── media.go            # 媒体响应构建
│   │   ├── transcoder.go       # FFmpeg 转码流处理
│   │   ├── hwaccel.go          # 硬件加速检测与配置
│   │   ├── probe.go            # 媒体探测 + FFmpeg 多路径发现
│   │   └── store.go            # 媒体数据库存储操作
│   ├── scanner/                # 文件扫描器
│   │   ├── scanner.go          # 目录遍历与文件分类
│   │   └── subtitle.go         # 字幕文件检测
│   ├── server/                 # 服务器 orchestration
│   │   └── server.go           # Server 结构体：配置管理、服务协调
│   ├── service/                # 业务逻辑层
│   │   ├── media.go            # MediaService 缓存协调
│   │   ├── session.go          # Session 管理服务
│   │   ├── logger.go           # 日志服务
│   │   └── config.go           # 配置业务逻辑
│   ├── storage/                # 数据持久化
│   │   ├── sqlite.go           # SQLite 数据库初始化与操作
│   │   ├── store.go            # Store 抽象层
│   │   └── interface.go        # 存储接口定义
│   ├── types/                  # 类型别名（向后兼容）
│   │   └── types.go            # domain 包的类型重导出
│   ├── util/                   # 工具函数
│   │   └── util.go             # 通用工具：路径处理、ID 编解码等
│   └── web/                    # Web 服务工具
│       └── web.go              # 嵌入式静态资源服务
├── web/                        # 前端应用
│   ├── src/
│   │   ├── app.js              # 应用入口
│   │   ├── app.css             # 全局样式
│   │   └── modules/            # 功能模块
│   │       ├── actions.js      # 应用启动与配置加载
│   │       ├── api.js          # API 请求封装
│   │       ├── state.js        # 全局状态管理
│   │       ├── eventbus.js     # 事件总线
│   │       ├── i18n.js         # 国际化
│   │       ├── theme.js        # 主题管理
│   │       ├── pin.js          # PIN 码验证
│   │       ├── lyrics.js       # 歌词显示
│   │       ├── icons.js        # 统一 SVG 图标库（icon(name) 单出口）
│   │       ├── utils.js        # 前端工具函数
│   │       ├── player/         # 播放器模块
│   │       │   ├── index.js    # 播放器入口（re-export + bus 监听）
│   │       │   ├── play.js     # 播放编排（playItem + getPlaybackUrl + onMediaEnded）
│   │       │   ├── core.js     # 核心播放逻辑（Plyr、媒体元素管理）
│   │       │   ├── seek.js     # 进度控制
│   │       │   ├── transcode.js# 转码回退（错误兜底）
│   │       │   ├── audio-track.js # 音轨切换
│   │       │   └── resume.js   # 续播功能
│   │       ├── playlist/       # 播放列表模块
│   │       │   ├── index.js    # 播放列表入口
│   │       │   ├── navigation.js # 导航控制
│   │       │   ├── sort-filter.js # 排序过滤
│   │       │   └── render.js   # 列表渲染
│   │       └── ui/             # UI 模块
│   │           ├── index.js    # UI 入口
│   │           ├── bindings.js # 事件绑定
│   │           ├── render.js   # 界面渲染
│   │           ├── pager.js    # 通用分页器组件（文件列表/播放列表共用）
│   │           ├── settings.js # 设置面板
│   │           └── shares.js   # 共享目录管理
│   ├── public/                 # 静态资源
│   │   └── assets/             # 第三方库（Plyr、pinyin-pro）
│   ├── index.html              # HTML 模板
│   └── vite.config.js          # Vite 配置
├── scripts/                    # 构建脚本
│   ├── dev.ps1 / dev.sh        # 开发服务器
│   └── build.ps1 / build.sh    # 生产构建
├── docs/                       # 文档
│   └── release/                # 版本发布说明
├── .github/workflows/          # CI/CD
│   ├── check.yml               # PR 检查
│   └── release.yml             # 发布流程
├── web/embed.go                # 前端资源嵌入（go:embed）
├── config.example.json         # 配置示例
├── go.mod                      # Go 模块定义
└── Dockerfile                  # 容器构建
```

---

## 3. 架构分层

### 3.1 后端架构（Clean Architecture 风格）

```
┌─────────────────────────────────────────────────────────────┐
│                        Entry Point                          │
│                     cmd/msp/main.go                         │
│         (路由注册、依赖注入、优雅关闭、信号处理)              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Handler Layer                       │
│                   internal/handler/                         │
│    (请求解析、参数验证、响应序列化、HTTP 状态码管理)          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Service Layer                             │
│                  internal/service/                          │
│       (业务逻辑编排、会话管理、日志服务、配置服务)            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Domain Layer                              │
│                  internal/domain/                           │
│            (实体定义、值对象、领域常量)                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Infrastructure / Repository Layer              │
│     ┌──────────────┬──────────────┬──────────────┐         │
│     │   internal/  │  internal/   │  internal/   │         │
│     │    cache/    │   storage/   │    media/    │         │
│     │  (内存缓存)   │   (SQLite)   │  (FFmpeg)    │         │
│     └──────────────┴──────────────┴──────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 前端架构（模块化设计）

```
┌─────────────────────────────────────────────────────────────┐
│                      Entry Point                            │
│                     src/app.js                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Action Layer                             │
│                 src/modules/actions.js                      │
│       (应用启动、配置加载、媒体数据获取、初始化流程)          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Feature Modules                           │
│  ┌──────────────┬──────────────┬──────────────┐            │
│  │   player/    │  playlist/   │     ui/      │            │
│  │   播放器      │   播放列表    │   界面控制    │            │
│  └──────────────┴──────────────┴──────────────┘            │
│  ┌──────────────┬──────────────┬──────────────┐            │
│  │    api.js    │   state.js   │  eventbus.js │            │
│  │   API 封装   │   状态管理    │   事件总线    │            │
│  └──────────────┴──────────────┴──────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. 核心流程

### 4.1 应用启动流程

```
main.go
    │
    ├──► 设置 GC 参数、日志格式
    │
    ├──► 初始化 IDCodec
    │       └── util.NewIDCodec(key) — 加载/生成 msp.key
    │           └── 确定性 nonce：HMAC-SHA256(key, path)[:gcm.NonceSize()]
    │
    ├──► 创建 Server 实例
    │       └── New(cfgPath, processor)
    │           ├── 初始化 SessionService
    │           ├── 初始化 LoggerService
    │           └── 初始化 MediaService (含 MediaCache + MediaProcessor)
    │
    ├──► 加载或初始化配置 LoadOrInitConfig()
    │
    ├──► 设置日志 SetupLogger()
    │
    ├──► FFmpeg 路径发现 + 硬件加速检测 initHWAccel(processor)
    │       ├── processor.CheckFFmpeg()
    │       │       └── resolveFFmpegPaths() — 7 层搜索
    │       │           ├── MSP_FFMPEG_PATH 环境变量
    │       │           ├── 可执行文件同目录 / bin/ 子目录
    │       │           ├── 当前工作目录 / bin/ 子目录
    │       │           ├── 平台特定路径
    │       │           └── 系统 PATH
    │       └── processor.MediaProcessor.DetectHWAccel(mode)
    │           ├── 探测 NVENC/QSV/AMF/VAAPI/VideoToolbox
    │           └── 返回可用的硬件编码器（无 FFmpeg 时并发上限归零）
    │
    ├──► 初始化数据库 InitSQLite()
    │
    ├──► 启动配置热重载 WatchConfig()
    │
    ├──► 预加载媒体缓存 GetOrBuildMediaCache()
    │
    ├──► 注册 HTTP 路由 registerRoutes()
    │       ├── /api/config   -> HandleConfig
    │       ├── /api/shares   -> HandleShares
    │       ├── /api/media    -> HandleMedia
    │       ├── /api/stream   -> HandleStream
    │       ├── /api/subtitle -> HandleSubtitle
    │       ├── /api/probe    -> HandleProbe
    │       ├── /api/progress -> HandleProgress
    │       ├── /api/prefs    -> HandlePrefs
    │       ├── /api/log      -> HandleLog
    │       ├── /api/pin      -> HandlePIN
    │       └── /             -> ServeEmbeddedWeb
    │
    ├──► 启动 HTTP 服务器
    │
    └──► 优雅关闭处理
            ├── 停止接受新连接
            ├── 终止转码进程 processor.KillAllTranscodeProcesses()
            ├── 关闭数据库连接
            └── 关闭日志文件
```

### 4.2 媒体数据获取流程

```
客户端请求 /api/media
    │
    ▼
Handler.HandleMedia()
    │
    ├──► 获取配置中的 shares 和 blacklist
    │
    ├──► 调用 MediaService.GetOrBuildMediaCache()
    │       │
    │       └──► cache.MediaCache.GetOrBuild()
    │               │
    │               ├──► 检查内存缓存（key 匹配 + TTL 有效）
    │               │       ├── 命中 -> 返回缓存数据
    │               │       └── 未命中 -> 继续
    │               │
    │               ├──► 检查磁盘缓存（无数据库时）
    │               │       ├── 命中 -> 加载到内存 -> 返回
    │               │       └── 未命中 -> 继续
    │               │
    │               ├──► 检查数据库缓存（有数据库时）
    │               │       └── processor.LoadMediaFromDB()
    │               │
    │               └──► 重建缓存 buildAndUpdate()
    │                       │
    │                       ├──► 数据库可用?
    │                       │       ├── 是 -> processor.ReindexAndLoadMedia()
    │                       │       │       ├── 扫描文件系统
    │                       │       │       ├── 写入数据库
    │                       │       │       └── 返回结果
    │                       │       └── 否 -> media.BuildMediaResponse(idCodec)
    │                       │               └── scanner.WalkShares(idCodec)
    │                       │                       ├── 遍历共享目录
    │                       │                       ├── 过滤黑名单
    │                       │                       ├── 分类文件类型
    │                       │                       └── 排序返回
    │                       │
    │                       ├──► 更新内存缓存
    │                       ├──► 保存到磁盘（无数据库时）
    │                       └──► 返回响应 + ETag
    │
    ├──► 应用 limit 参数（如指定）
    │
    └──► 返回 JSON 响应（支持 304 Not Modified）
```

### 4.3 流媒体传输流程

```
客户端请求 /api/stream?id=xxx&transcode=1
    │
    ▼
Handler.HandleStream()
    │
    ├──► 解析并验证媒体 ID
    │       └── h.idCodec.DecodeID() -> 文件路径
    │
    ├──► 安全验证 util.IsAllowedFile()
    │       └── 确保文件在共享目录内
    │
    ├──► 检查转码策略 checkTranscodePolicy()
    │       ├── 查询配置是否允许转码
    │       └── 验证文件类型（视频/音频）
    │
    ├──► 需要转码?
    │       │
    │       ├── 是 -> tryServeTranscode()
    │       │       │
    │       │       ├──► processor.TranscodeStream()
    │       │       │       │
    │       │       │       ├──► 检查 processor.FFmpegPath() 是否可用
    │       │       │       │
    │       │       │       ├──► 验证转码选项
    │       │       │       │
    │       │       │       ├──► 获取信号量（并发控制）
    │       │       │       │
    │       │       │       ├──► 探测源文件编码 processor.GetCodecInfo()
    │       │       │       │
    │       │       │       ├──► 智能选择转码策略
    │       │       │       │       ├── H.264 视频 -> copy 视频流
    │       │       │       │       ├── 其他视频 -> 转码（硬件/软件）
    │       │       │       │       ├── AAC/MP3 音频 -> copy 音频流
    │       │       │       │       └── 其他音频 -> 转 AAC
    │       │       │       │
    │       │       │       ├──► 构建 FFmpeg 参数
    │       │       │       │       └── processor.BuildVideoArgs()
    │       │       │       │           ├── 硬件加速可用?
    │       │       │       │           │       ├── 是 -> 使用 h264_nvenc/qsv/amf 等
    │       │       │       │           │       └── 否 -> 使用 libx264
    │       │       │       │
    │       │       │       ├──► 启动 FFmpeg 进程
    │       │       │       ├──► 返回 stdout 流
    │       │       │       └──► 后台等待进程结束
    │       │       │
    │       │       └──► 流式传输到客户端
    │       │
    │       └── 否 -> serveDirect()
    │               └── http.ServeContent() (支持 Range 请求)
    │
    └──► 清理资源
```

### 4.3.1 播放策略决策流程（v1.2.0+）

```
客户端请求 /api/probe?id=xxx
    │
    ▼
Handler.HandleProbe()
    │
    ├──► 解析并验证媒体 ID
    │
    ├──► 字节嗅探编码信息 scanner.SniffContainerCodecs()
    │       ├── 读取文件首尾各 2MB
    │       ├── MKV: 匹配 V_MPEGH/ISO/HEVC 等编码标签
    │       └── MP4/M4V/MOV: 匹配 hvc1/avc1 等 FourCC
    │
    ├──► 检查转码配置是否开启
    │       └── playback.video.transcode / playback.audio.transcode
    │
    ├──► decidePlaybackMode(videoCodec, audioCodec, ffmpegAvailable)
    │       │
    │       ├── 无 FFmpeg → "direct"（只能直连）
    │       │
    │       ├── 视频编码判断
    │       │       ├── H.264/AVC → 继续检查音频
    │       │       ├── H.265/HEVC → "transcode"
    │       │       ├── AV1 → "transcode"
    │       │       ├── VC-1/WMV3 → "transcode"
    │       │       └── 未知编码 → "transcode"（保守）
    │       │
    │       ├── 音频编码判断（解决"有画无声"）
    │       │       ├── AAC/MP3/Opus/Vorbis/FLAC → 继续
    │       │       ├── AC-3/DTS/TrueHD → "transcode"
    │       │       └── 未知编码 → "transcode"（保守）
    │       │
    │       └── 全部兼容 → "direct"
    │
    └──► 返回 ProbeResponse（含 playback.mode）
```

### 4.4 前端初始化流程

```
app.js: boot()
    │
    ├──► 注册 Service Worker
    │
    ├──► 初始化国际化 initLang()
    │
    ├──► 初始化主题 initTheme()
    │
    ├──► 绑定全局快捷键 bindGlobalHotkeys()
    │
    ├──► 绑定 PIN 码对话框 bindPinDialog()
    │
    ├──► 检查是否需要 PIN 码 checkPinRequired()
    │       └── 需要 -> 显示 PIN 对话框，暂停启动
    │
    ├──► 加载配置 loadConfig()
    │       └── GET /api/config
    │
    ├──► 加载用户偏好 loadPrefs()
    │       └── GET /api/prefs
    │
    ├──► 快速加载媒体（有限数量）loadMedia(limit=200)
    │       └── GET /api/media?limit=200
    │
    └──► 后台完整加载
            └── setTimeout -> loadMedia()
                    └── GET /api/media
                    └── 轮询更新（如正在扫描）
```

---

## 5. 核心数据结构

### 5.1 领域模型 (internal/domain/types.go)

```go
// MediaItem - 媒体文件实体
type MediaItem struct {
    ID         string     // 路径的 Base64 编码（URL Safe）
    Path       string     // 绝对路径（不序列化到 JSON）
    Name       string     // 文件名
    Ext        string     // 扩展名（小写）
    Kind       string     // 类型: video/audio/image/other
    ShareLabel string     // 所属共享目录标签
    Size       int64      // 文件大小（字节）
    ModTime    int64      // 修改时间戳
    Subtitles  []Subtitle // 外挂字幕列表
    CoverID    string     // 音频封面图 ID
    LyricsID   string     // 歌词文件 ID
    ScanID     int64      // 扫描批次 ID
}

// PlaybackProgress - 播放进度
type PlaybackProgress struct {
    MediaID   string    // 媒体 ID
    Time      float64   // 播放位置（秒）
    UpdatedAt time.Time // 更新时间
}

// MediaResponse - 媒体列表响应
type MediaResponse struct {
    Shares      []Share     // 共享目录列表
    Videos      []MediaItem // 视频列表
    Audios      []MediaItem // 音频列表
    Images      []MediaItem // 图片列表
    Others      []MediaItem // 其他文件列表
    VideosTotal int         // 视频总数（分页用）
    AudiosTotal int         // 音频总数
    ImagesTotal int         // 图片总数
    OthersTotal int         // 其他文件总数
    Limited     bool        // 是否被限制返回数量
    Scanning    bool        // 是否正在扫描
}

// PlaybackStrategy - 播放策略（v1.2.0+）
type PlaybackStrategy struct {
    Mode string // "direct" 或 "transcode"
}

// ProbeResponse - 媒体探针响应
type ProbeResponse struct {
    Container string            // 容器格式（如 "mkv"、"mp4"）
    Video     string            // 视频编码（如 "H.264/AVC"、"H.265/HEVC"）
    Audio     string            // 音频编码（如 "AAC"、"AC-3"）
    Subtitles []Subtitle        // 外挂字幕列表
    Playback  *PlaybackStrategy // 推荐播放策略（转码开启时返回）
    Error     *ApiError         // 错误信息
}
```

### 5.2 配置结构 (internal/config/config.go)

```go
type Config struct {
    Port     int           // 服务端口（默认 8099）
    Shares   []Share       // 共享目录列表
    Features FeatureConfig // 功能开关
    UI       UIConfig      // UI 配置
    Playback PlaybackConfig // 播放配置
    Blacklist BlacklistConfig // 黑名单配置
    Security SecurityConfig // 安全配置
    LogLevel string        // 日志级别
    LogFile  string        // 日志文件路径
    MaxItems int           // 最大扫描文件数
}
```

---

## 6. 关键接口与依赖关系

### 6.1 Handler 依赖接口 (internal/handler/handler.go)

```go
// ConfigProvider - 配置提供器
type ConfigProvider interface {
    Config() config.Config
    UpdateConfig(func(*config.Config)) error
    GetPort() int
}

// MediaCacheProvider - 媒体缓存提供器
type MediaCacheProvider interface {
    GetOrBuildMediaCache(ctx context.Context, shares []domain.Share, blacklist config.BlacklistConfig, refresh bool) (domain.MediaResponse, string)
    InvalidateMediaCache()
}

// SessionProvider - 会话管理提供器
type SessionProvider interface {
    CreateSession() (string, error)
    ValidateSession(token string) bool
}

// Logger - 日志接口
type Logger interface {
    Log(level string, msg string)
    LogRequest(r *http.Request, status int, start time.Time)
}
```

### 6.2 存储接口 (internal/storage/interface.go)

```go
// ProgressStore - 播放进度存储
type ProgressStore interface {
    GetProgress(ctx context.Context, mediaID string) (float64, error)
    SetProgress(ctx context.Context, mediaID string, t float64) error
}

// PrefsStore - 用户偏好存储
type PrefsStore interface {
    GetAllPrefs(ctx context.Context) (map[string]string, error)
    SetPrefs(ctx context.Context, kv map[string]string) error
}
```

---

## 7. 重要实现细节

### 7.1 硬件加速检测逻辑 (internal/media/hwaccel.go)

```
DetectHWAccel(mode)
    │
    ├── mode == "none" -> 禁用硬件加速
    │
    ├── 获取平台相关的候选编码器列表
    │       ├── NVENC (NVIDIA) - 全平台
    │       ├── QSV (Intel) - 全平台
    │       ├── AMF (AMD) - Windows
    │       ├── VAAPI - Linux
    │       └── VideoToolbox - macOS
    │
    ├── 如指定特定模式，过滤候选列表
    │
    └── 逐一探测编码器可用性
            └── probeEncoder()
                    ├── 构建测试 FFmpeg 命令
                    ├── 5 秒超时执行
                    └── 成功 -> 返回可用编码器
```

### 7.2 文件扫描过滤逻辑 (internal/scanner/scanner.go)

```
WalkShares(ctx, shares, blacklist, limit, callback, idCodec)
    │
    ├── 遍历每个共享目录
    │
    ├── 使用 filepath.WalkDir 遍历文件
    │
    ├── 目录过滤 ShouldSkipDir()
    │       ├── 隐藏目录（以 . 开头）
    │       └── 黑名单中的文件夹名
    │
    ├── 文件过滤 shouldSkipFile()
    │       ├── 无扩展名
    │       ├── 黑名单扩展名
    │       ├── 黑名单文件名
    │       ├── 字幕/歌词文件
    │       └── 大小规则过滤
    │
    └── 构建 MediaItem 并回调
            ├── 生成 ID（AES-GCM 加密路径，确定性 nonce）
            ├── 分类文件类型 ClassifyExt()
            ├── 视频文件 -> 查找外挂字幕
            └── 音频文件 -> 查找封面和歌词
```

### 7.3 转码流控制 (internal/media/transcoder.go)

```
MediaProcessor.TranscodeStream(ctx, inputPath, opts)
    │
    ├── 验证转码选项（格式、码率、偏移量）
    │
    ├── 验证输入文件（存在、可读、常规文件）
    │
    ├── 获取信号量（全局并发限制，默认 2-4）
    │
    ├── 探测源文件编码信息
    │
    ├── 智能选择转码策略
    │       ├── 音频模式（MP3/AAC）
    │       │       └── 格式匹配则 copy，否则转码
    │       └── 视频模式（MP4）
    │               ├── H.264 视频 -> copy
    │               ├── 其他视频 -> 转 H.264（硬件优先）
    │               ├── AAC/MP3 音频 -> copy
    │               └── 其他音频 -> 转 AAC
    │
    ├── 构建 FFmpeg 参数
    │       ├── 硬件加速初始化参数（如可用）
    │       ├── 输入文件和偏移量
    │       ├── 视频编码参数（硬件/软件）
    │       ├── 音频编码参数
    │       └── MP4 优化参数（faststart）
    │
    ├── 启动 FFmpeg 进程
    │
    ├── 注册到活跃进程表（用于优雅关闭）
    │
    └── 返回包装后的 stdout 流（带信号量释放）
```

---

## 8. API 端点清单

| 端点 | 方法 | 功能 | 关键参数 |
|------|------|------|----------|
| `/api/config` | GET/POST | 获取/更新配置 | - |
| `/api/shares` | POST | 共享目录操作（增删改） | op, label, path |
| `/api/media` | GET | 获取媒体列表 | refresh, limit |
| `/api/stream` | GET/HEAD | 流媒体传输 | id, transcode, format, bitrate, start |
| `/api/subtitle` | GET | 获取字幕内容 | id |
| `/api/probe` | GET | 探测媒体信息 | id |
| `/api/progress` | GET/POST | 获取/更新播放进度 | mediaId, time |
| `/api/prefs` | GET/POST | 获取/更新用户偏好 | prefs |
| `/api/log` | POST | 客户端日志上报 | level, msg |
| `/api/pin` | GET/POST | PIN 码验证 | pin |
| `/api/ip` | GET | 获取服务器 IP | - |

---

## 9. 配置热重载机制

```
Server.WatchConfig()
    │
    ├── 每 5 秒检查配置文件修改时间
    │
    ├── 检测到修改?
    │       │
    │       └── 是 -> 读取并解析配置
    │               ├── 应用默认值
    │               ├── 更新内存配置
    │               ├── 更新日志配置
    │               └── 记录重载日志
    │
    └── 循环直到上下文取消
```

---

## 10. 安全机制

### 10.1 文件访问控制

```go
// util.IsAllowedFile(target, shares)
// 确保请求的文件路径在共享目录范围内

1. 解析目标文件的绝对路径
2. 遍历每个共享目录
3. 检查目标路径是否以共享目录路径开头
4. 返回是否允许访问
```

### 10.2 会话与 PIN 验证

```
HandlePIN()
    ├── GET -> 返回 PIN 是否启用
    └── POST -> 验证 PIN 码
            ├── 匹配 -> 创建 Session Cookie
            └── 不匹配 -> 返回 401

中间件验证（WithSecurity）
    ├── 检查路径是否在白名单
    ├── 验证 Session Cookie
    └── 无效 -> 返回 401
```

### 10.3 IP 黑白名单

```
WithSecurity 中间件
    ├── 提取客户端 IP（考虑 X-Forwarded-For）
    ├── 检查 IP 黑名单
    ├── 检查 IP 白名单（如配置）
    └── 拒绝 -> 返回 403
```

---

## 11. 构建与部署

### 11.1 开发环境

```powershell
# Windows
.\scripts\dev.ps1 [-BackendPort 3000]

# Linux/Mac
./scripts/dev.sh [--backend-port 3000]
```

### 11.2 生产构建

```powershell
# Windows
.\scripts\build.ps1

# Linux/Mac
./scripts/build.sh
```

### 11.3 Docker 构建

```dockerfile
# 多阶段构建
1. 前端构建（node:22-alpine）
2. 后端编译（golang:1.24-alpine）
3. 运行时（alpine）
```

---

## 12. 测试策略

| 类型 | 位置 | 说明 |
|------|------|------|
| 单元测试 | `*_test.go` | 各包的单元测试 |
| 集成测试 | `internal/handler/*_test.go` | HTTP 处理器测试 |
| 安全测试 | `internal/handler/*_safety_test.go` | 安全相关测试 |
| 前端测试 | `web/web_test.go` | 仅验证嵌入 FS 编译 |

---

## 13. 扩展点

### 13.1 添加新的媒体处理器

1. 在 `internal/scanner/scanner.go` 的 `ClassifyExt()` 中添加扩展名映射
2. 在 `internal/handler/stream.go` 的 `contentTypeByExt` 中添加 MIME 类型

### 13.2 添加新的 API 端点

1. 在 `internal/handler/` 创建处理器方法
2. 在 `cmd/msp/main.go` 的 `registerRoutes()` 中注册路由
3. 如需持久化，在 `internal/storage/sqlite.go` 添加数据库操作

### 13.3 添加新的硬件编码器支持

1. 在 `internal/media/hwaccel.go` 的 `hwCandidates()` 中添加编码器配置
2. 实现平台特定的初始化参数和编码参数

---

## 14. 调试与监控

### 日志级别

- `debug` - 详细调试信息
- `info` - 常规操作日志（默认）
- `warn` - 警告信息
- `error` - 错误信息

### 关键指标

- 转码并发数（通过信号量控制）
- 媒体缓存命中率
- 数据库查询性能（GORM 慢查询日志阈值 2s）

---

*文档版本: 1.1*
*最后更新: 2026-05-09*
