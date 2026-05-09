# MSP 模块化重构计划

## 重构概览

| Phase | 描述 | 状态 |
|-------|------|------|
| Phase 0 | 准备工作 - domain 包提取 | ✅ 已完成 |
| Phase 1 | 拆分后端大文件 | ✅ 已完成 |
| Phase 2 | 引入接口 + 依赖注入 | ✅ 已完成 |
| Phase 3 | 前端解耦 | ✅ 已完成 |
| Phase 4 | 收尾优化 | ✅ 已完成 |

---

## Phase 0: 准备工作 ✅ 已完成

| 步骤 | 改动 | 验证 |
|------|------|------|
| 0a | 创建 `domain/` 包，将 `Share` 类型从 `config` 移到 `domain` | `go build` ✅ |
| 0b | 将 `types.go` 中所有类型移入 `domain/`，`MediaResponse.Shares` 改为 `[]domain.Share`，`types` 包保留为 type alias 垫片 | `go build` ✅ |
| 0c | `util` 包的 `DedupeShares`/`NormalizeShares`/`IsAllowedFile` 参数从 `[]config.Share` 改为 `[]domain.Share` | `go build` + `go test` + `go vet` ✅ |

**变更文件清单：**
- 新增: `internal/domain/share.go`, `internal/domain/types.go`
- 修改: `internal/config/config.go` (移除 Share 定义, 引用 domain.Share)
- 修改: `internal/config/validate.go` (validateShares 参数改 domain.Share)
- 修改: `internal/config/validate_test.go` (domain.Share)
- 修改: `internal/types/types.go` (改为 type alias 垫片)
- 修改: `internal/util/util.go` (domain.Share)
- 修改: `internal/util/util_test.go` (domain.Share)
- 修改: `internal/service/config.go` (domain.Share)
- 修改: `internal/service/config_test.go` (domain.Share)
- 修改: `internal/media/media.go` (domain.Share + domain.MediaResponse)
- 修改: `internal/media/scanner.go` (domain 类型)
- 修改: `internal/media/scanner_test.go` (domain 类型)
- 修改: `internal/media/store.go` (domain 类型)
- 修改: `internal/server/server.go` (domain 类型)
- 修改: `internal/handler/handlers.go` (domain 类型)
- 修改: `internal/handler/handlers_safety_test.go` (domain 类型)
- 修改: `internal/handler/config_test.go` (domain 类型)
- 修改: `internal/db/db.go` (domain 类型)
- 修改: `internal/db/db_test.go` (domain 类型)

---

## Phase 1: 拆分后端大文件 ✅ 已完成

**原则：每步仅移动代码，不改逻辑。拆完即可编译运行。**

### 1a: 拆分 `handler/handlers.go` (720行) ✅

拆为 8 个文件：
- `handler/handler.go` — Handler 结构体 + New() + 常量 (defaultJSONBodyLimit, maxSubtitleConvertSize)
- `handler/stream.go` — HandleStream + HandleProbe + resolveMediaTarget + checkTranscodePolicy + tryServeTranscode + serveDirect + determineContentType + contentTypeByExt
- `handler/media.go` — HandleMedia + parseLimitParam + applyLimit + writeNotModifiedIfMatch
- `handler/config.go` — HandleConfig + HandleShares + normalizeSharesOp + applySharesOp + handleShareAdd + handleShareRemove + HandleIP
- `handler/auth.go` — HandlePIN + constantTimeCompare
- `handler/subtitle.go` — HandleSubtitle + serveVTT + serveSRT + serveASS
- `handler/progress.go` — HandlePrefs + HandleProgress + HandleLog
- `handler/common.go` — writeJSON + decodeJSONBody + isPayloadTooLarge + writeJSONDecodeError
- `handler/middleware.go` — 不变

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅

### 1b: 拆分 `media/scanner.go` (869行) → `internal/scanner/` 包 ✅

- 新增 `scanner/scanner.go` — WalkShares + shareWalker + buildMediaItem + ShouldSkipDir + shouldSkipFile + ClassifyExt + IsBlockedString + IsBlockedSize + SniffContainerCodecs + 嗅探函数和表
- 新增 `scanner/subtitle.go` — FindSidecarSubtitles/Cached + FindAudioSidecarsCached + SubtitleLabel + subtitleLabelMap + collectSubtitles + sortSubtitles + normalizeBaseForMatch + extractBaseVariants + IsSubtitleExt + IsLyricsExt + SrtToVtt + AssToVtt + IsAllDigits + findLyrics + findCover + lrcPicker
- 新增 `scanner/scanner_test.go` — 原 scanner_test.go 的测试（移除 WithinRoot 测试，已在 util_test.go 中）
- 修改 `media/media.go` — 改为 import `scanner.WalkShares`
- 修改 `media/store.go` — 改为 import `scanner.WalkShares`
- 修改 `handler/stream.go` — 改为 import `scanner.ClassifyExt` + `scanner.SniffContainerCodecs` + `scanner.FindSidecarSubtitles`
- 修改 `handler/subtitle.go` — 改为 import `scanner.SrtToVtt` + `scanner.AssToVtt`
- 删除 `media/scanner.go` 和 `media/scanner_test.go`

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅

### 1c: 从 `media/transcoder.go` 拆出 `probe.go` ✅

- 新增 `media/probe.go` — CodecInfo struct + CheckFFmpeg + CheckFFprobe + GetCodecInfo
- 修改 `media/transcoder.go` — 移除上述函数

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅

### 1d: 将 `server.go` 缓存逻辑提取到 `cache/media.go` ✅

- 新增 `cache/media.go` — MediaCache struct + GetOrBuild + Invalidate + buildAndUpdate + rebuild + LoadFromDisk + saveToDisk + CacheKey + WeakETag + sharesCacheKey + normRuleList + mediaCacheOnDisk + FormatMediaCachePath
- 修改 `server.go` — 媒体缓存字段替换为 `mediaCache *cache.MediaCache`，InvalidateMediaCache/GetOrBuildMediaCache/LoadMediaCacheFromDisk 委托给 cache 包；移除所有缓存实现代码
- `server.go` 从 626行 减至 ~310行

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅

---

## Phase 2: 引入接口 + 依赖注入 ✅ 已完成

### 2a: 创建 `storage/` 包（接口 + SQLite 实现 + Store 委托）✅

- 新增 `storage/interface.go` — `ProgressStore` 接口（GetProgress, SetProgress）+ `PrefsStore` 接口（GetAllPrefs, SetPrefs）
- 新增 `storage/sqlite.go` — `SQLite` 结构体，拥有 `gorm.DB`，实现 `ProgressStore`、`PrefsStore` 接口 + 全部媒体相关数据库操作（GetScanMeta, SetScanMeta, UpsertMediaItem, UpsertMediaItems, DeleteStaleByScan, DeleteByShareRootsNotIn, QueryMediaItems, CountMediaItems, ByScan, ByKind）+ InitSQLite 构造函数 + Close + DB()
- 新增 `storage/store.go` — `Store` 结构体（nil-safe 包装 `*SQLite`），实现 `ProgressStore` 和 `PrefsStore`，redis-nullable nil 的进度/偏好查询
- 新增 `storage/sqlite_test.go` — 完整数据库测试（从 db_test.go 迁移并适配）
- 删除 `db/db.go` 和 `db/db_test.go` — 全局 `var DB *gorm.DB` 已移除

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 2b: Handler 依赖接口而非 `*server.Server` ✅

- 在 `handler/handler.go` 中定义接口：
  - `ConfigProvider` — Config(), UpdateConfig(), InvalidateMediaCache(), GetPort()
  - `MediaCacheProvider` — GetOrBuildMediaCache(ctx, shares, blacklist, refresh)
  - `SessionProvider` — CreateSession(), ValidateSession(token)
  - `Logger` — Log(level, msg), LogRequest(r, status, start)
- `Handler` 结构体改为引用 6 个接口：`ConfigProvider`, `MediaCacheProvider`, `SessionProvider`, `Logger`, `storage.ProgressStore`, `storage.PrefsStore`
- 新增 `Deps` 结构体作为 `New()` 参数，替代 `*server.Server`
- `service/config.go` 改为依赖 `ConfigProvider` 接口（不再 import server 包）
- `handler/middleware.go` — `WithLog(logger Logger, next)` 和 `WithSecurity(config, session, logger, next)` 替代原 `WithLog(s *server.Server, next)`
- 所有 handler 方法改为 `h.config.Config()`, `h.media.GetOrBuildMediaCache()`, `h.session.CreateSession()`, `h.logger.Log()` 等
- `handler/progress.go` — `db.GetProgress/SetProgress/GetAllPrefs/SetPrefs` 替换为 `h.progress.GetProgress/SetProgress` 和 `h.prefs.GetAllPrefs/SetPrefs`
- `cmd/msp/main.go` — 更新路由注册和中间件调用，注入 `s, s, s, s, store, store` 作为接口实现
- 编译时接口断言：`*server.Server` 实现 `ConfigProvider/MediaCacheProvider/SessionProvider/Logger`；`*storage.SQLite` 实现 `ProgressStore/PrefsStore`

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 2c: `db.go` → `storage/sqlite.go`，删除全局 `db.DB` ✅

- 新增 `storage/sqlite.go` — `SQLite` 结构体封装 `gorm.DB`，所有数据库操作转为方法
- 新增 `storage/store.go` — `Store` nil-safe 包装（测试传入 nil 不 panic）
- 修改 `media/store.go` — 改为使用 `storage.SQLite` 全局变量 `mediaDB`，新增 `SetDB()` 和 `IsDBAvailable()`
- 修改 `cache/media.go` — `db.DB != nil` 替换为 `media.IsDBAvailable()`
- 修改 `cmd/msp/main.go` — `db.Init()` → `storage.InitSQLite()`；`db.Close()` → `sq.Close()`；`media.SetDB(sq)`
- 删除 `db/db.go` 和 `db/db_test.go`

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 2d: 将 `server.go` 会话管理提取到 `service/session.go` ✅

- 新增 `service/session.go` — `SessionService` 结构体（`sync.RWMutex` + `sessions map`），实现 `CreateSession()` 和 `ValidateSession()`
- 修改 `server.go` — 移除 `sessionMu`/`sessions` 字段和 `CreateSession`/`ValidateSession`/`cleanupExpiredSessionsLocked` 方法，新增 `session *service.SessionService` 字段，`New()` 中创建 `service.NewSessionService()`
- `server.Server.CreateSession()` 和 `ValidateSession()` 委托给 `s.session`
- 编译时断言：`*service.SessionService` 实现 `handler.SessionProvider`
- 修改 `service/config_test.go` — 使用 `mockConfigProvider` 替代 `*server.Server`（消除循环 import）

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 2e: `service/media.go` 编排层 ✅

- 新增 `service/media.go` — `MediaService` 结构体，封装 `MediaCache` 和配置读取
- 实现 `GetOrBuildMediaCache()` — 从 config 获取 MaxItems 后调用 cache
- 实现 `InvalidateMediaCache()` — 委托给 cache.Invalidate()
- 实现 `LoadMediaCacheFromDisk()` — 从 config 获取 Shares 和 Blacklist 后调用 cache
- 修改 `server.go` — 使用 `MediaSvc *service.MediaService` 字段，媒体相关方法委托给 service
- 修改 `cmd/msp/main.go` — handler 注入 `s.MediaSvc` 作为 `MediaCacheProvider`

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

---

## Phase 3: 前端解耦 ✅ 已完成

### 3a: 创建 `eventbus.js` ✅

- 新增 `web/src/modules/eventbus.js` — 简单发布/订阅事件总线
- 实现 `bus.on(event, fn)`、`bus.off(event, fn)`、`bus.emit(event, data)`

### 3b: 断开 `player↔playlist` 循环 ✅

- `playlist.js` 不再直接 import `playItem` from `player.js`
- 改为 `bus.emit('play:request', item, opts)` 发送播放请求
- `player.js` 通过 `bus.on('play:request', ...)` 监听并调用 `playItem()`
- `player.js` 通过 `bus.emit('playlist:updated')` 通知列表更新

### 3c: 断开 `ui↔actions` 循环 ✅

- `actions.js` 不再直接 import `renderList`/`applyConfigToUI` from `ui.js`
- 改为 `bus.emit('config:loaded')`、`bus.emit('media:loaded')`、`bus.emit('boot:init')`
- `ui.js` 通过 `bus.on()` 监听这些事件并调用相应函数

### 3d: 拆分 `player.js` (1295行) ✅

| 文件 | 职责 |
|------|------|
| `player/index.js` | 统一导出所有模块 |
| `player/core.js` | Plyr 初始化/销毁/错误恢复/控件设置 |
| `player/resume.js` | 续播逻辑、全局快捷键绑定 |
| `player/transcode.js` | 转码 fallback + 重试逻辑 |
| `player/audio-track.js` | 多音轨处理、字幕轨道 |
| `player/seek.js` | 进度保存/心跳检测/恢复播放 |

### 3e: 拆分 `playlist.js` (456行) ✅

| 文件 | 职责 |
|------|------|
| `playlist/index.js` | 统一导出所有模块 |
| `playlist/sort-filter.js` | 排序/过滤/搜索逻辑 |
| `playlist/navigation.js` | 上一首/下一首/随机/播放列表生成 |
| `playlist/render.js` | 播放列表渲染、分页、自适应 |

### 3f: 拆分 `ui.js` (416行) ✅

| 文件 | 职责 |
|------|------|
| `ui/index.js` | 统一导出所有模块 |
| `ui/render.js` | 列表渲染、语言更新、对话框控制 |
| `ui/settings.js` | 配置应用到UI、播放列表设置 |
| `ui/shares.js` | 共享目录渲染、黑名单UI更新 |
| `ui/bindings.js` | 事件绑定、按钮处理 |

**验证:** 前端构建 ✅

---

## Phase 4: 收尾优化 ✅ 已完成

### 4a: `server.go` 瘦身至 ~200行 ✅

- 新增 `service/logger.go` — `LoggerService` 结构体，封装所有日志相关功能
  - `SetupLogger()` — 日志初始化
  - `Log()` / `LogRequest()` — 日志记录
  - `RotateLogIfNeeded()` — 日志轮转
  - `Close()` — 优雅关闭日志文件
- 修改 `server.go` — 移除日志相关字段和方法，委托给 `LoggerService`
- `server.go` 从 312行 减至 199行

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 4b: 添加优雅关机 ✅

- 修改 `cmd/msp/main.go`：
  - 使用 `signal.NotifyContext()` 监听 `SIGINT`/`SIGTERM`
  - 新增 `shutdownGracefully()` 函数处理关机流程
  - 调用 `media.KillAllTranscodeProcesses()` 终止所有 FFmpeg 进程
  - 使用 `http.Server.Shutdown()` 等待在途请求完成（超时 10s）
  - 关闭数据库连接和日志文件

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 4c: `probe.go` 添加 ffprobe 结果缓存 ✅

- 修改 `media/probe.go`：
  - 使用 `sync.Map` 作为缓存存储
  - 缓存键：`filepath + mtime`（文件路径 + 修改时间）
  - 默认 TTL：5分钟
  - 新增 `SetProbeCacheTTL()` 和 `ClearProbeCache()` 接口

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 4d: 正则预编译 ✅

- 修改 `scanner/scanner.go`：
  - 预编译 `blockedStringRegex` 用于检测正则格式的黑名单规则
  - 新增 `IsBlockedStringRegex()` 和 `GetBlockedStringPattern()` 辅助函数

- 修改 `scanner/subtitle.go`：
  - 预编译 `normalizePatterns` 切片（44个正则表达式）
  - 预编译 `cleanTextRegex` 用于 ASS 字幕文本清理

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

### 4e: `cmd/msp/main.go` 重构为依赖组装器 ✅

- 使用 `context.WithCancel` 管理生命周期
- 清晰的依赖组装流程：
  1. 创建 Server
  2. 初始化数据库
  3. 注册路由
  4. 启动 HTTP 服务
  5. 监听信号并优雅关机

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅

---

## 目标结构

```
cmd/msp/main.go                    # 入口：组装依赖、启动服务
internal/
  config/
    config.go                       # 配置类型定义 + 默认值
    validate.go                     # 校验逻辑
    validate_test.go
  constants/
    constants.go                    # 常量
    errors.go                       # 错误消息
  domain/                           # ✅ 领域类型，零外部依赖
    share.go                        # Share
    types.go                        # MediaItem, MediaResponse, Subtitle 等全部领域类型
  storage/                          # ✅ 存储层
    interface.go                    # ProgressStore, PrefsStore 接口
    sqlite.go                       # SQLite 实现（原 db.go）
    sqlite_test.go                  # ✅ 数据库测试（原 db_test.go）
    store.go                        # ✅ nil-safe Store 包装
  scanner/                          # ✅ 从 media 拆出
    scanner.go                      # 文件扫描、编解码探测
    subtitle.go                     # 字幕匹配+格式转换
    scanner_test.go
  cache/                            # ✅ 媒体缓存
    media.go                        # 媒体缓存
  media/
    media.go                        # BuildMediaResponse
    store.go                        # 数据库存储逻辑（使用 storage.SQLite）
    transcoder.go                   # FFmpeg 转码流程
    hwaccel.go                      # 硬件加速
    probe.go                        # ✅ ffprobe 探测（从 transcoder 拆出）+ 结果缓存
  service/                          # ✅ 业务编排层
    config.go                       # ✅ 配置变更编排（依赖 ConfigProvider 接口）
    config_test.go                  # ✅ mock 测试（无循环依赖）
    session.go                      # ✅ 会话管理
    media.go                        # ✅ 媒体服务编排层
    logger.go                       # ✅ 日志服务（新增）
  handler/                          # ✅ 已拆分 + 接口注入
    handler.go                      # ✅ Handler 结构体 + 接口定义 + Deps + New()
    stream.go                       # 流媒体/转码处理
    media.go                        # 媒体列表/缓存
    config.go                       # 配置 CRUD
    auth.go                         # PIN 认证
    subtitle.go                     # 字幕转换端点
    progress.go                     # 播放进度/偏好/日志
    middleware.go                   # ✅ 中间件（接受接口而非 *server.Server）
    common.go                       # writeJSON 等辅助
  util/
    path.go                         # IsAllowedFile, WithinRoot 等
    encoding.go                     # ID 编解码
    network.go                      # GetLanIPv4s 等
    share.go                        # Share 去重/规范化
  web/                              # 嵌入式前端服务（保持不变）
  server/                           # HTTP 服务器壳
    server.go                       # ✅ 配置加载/热更新/日志委托/缓存委托/会话委托

web/src/modules/
  state.js                          # 简化状态对象
  eventbus.js                       # ✅ 发布/订阅事件总线
  api.js, i18n.js, theme.js, icons.js, pin.js, lyrics.js, utils.js  # 保持不变
  player/                           # ✅ 已拆分
    index.js                        # 统一导出
    core.js                         # Plyr 初始化/销毁/错误恢复
    resume.js                       # 续播逻辑/快捷键
    transcode.js                    # 转码 fallback
    audio-track.js                  # 多音轨处理
    seek.js                         # 进度保存/心跳检测
  playlist/                         # ✅ 已拆分
    index.js                        # 统一导出
    sort-filter.js                  # 排序/过滤/搜索
    navigation.js                   # 上一首/下一首/随机
    render.js                       # 播放列表渲染
  ui/                               # ✅ 已拆分
    index.js                        # 统一导出
    render.js                       # 列表渲染/语言更新
    settings.js                     # 配置应用到UI
    shares.js                       # 共享目录管理
    bindings.js                     # 事件绑定
  actions.js                        # ✅ 仅依赖 eventbus
```

---

## 依赖方向（重构后，全部单向无循环）

```
domain (零依赖)
  ↑
constants, config (仅依赖 domain/constants)
  ↑
util (仅依赖 domain/constants)
  ↑
storage (仅依赖 domain + gorm)
  ↑
scanner, media (依赖 storage + domain + config)
  ↑
cache, service (依赖 domain + config + scanner + media + storage)
  ↑
handler (仅依赖 storage 接口 + domain + config + constants)
  ↑
server (HTTP 壳，依赖 handler + service + cache + media)
  ↑
cmd/msp/main (组装入口，注入所有依赖)
```

---

## 新增/修改文件汇总

### Phase 4 新增文件

| 文件 | 描述 |
|------|------|
| `internal/service/logger.go` | 日志服务，从 server.go 提取 |

### Phase 4 修改文件

| 文件 | 修改内容 |
|------|----------|
| `internal/server/server.go` | 瘦身至 199 行，日志委托给 LoggerService |
| `internal/config/config.go` | 添加 DefaultPort 等常量 |
| `internal/media/probe.go` | 添加 ffprobe 结果缓存（sync.Map + TTL） |
| `internal/media/transcoder.go` | 添加进程跟踪和 KillAllTranscodeProcesses |
| `internal/scanner/scanner.go` | 预编译正则表达式 |
| `internal/scanner/subtitle.go` | 预编译正则表达式 |
| `cmd/msp/main.go` | 添加优雅关机，重构为依赖组装器 |

### 脚本和工作流更新 ✅

- 更新 `scripts/build.ps1` / `scripts/build.sh`:
  - 添加 `-SkipTests` 和 `-SkipLint` 参数支持快速构建
  - 添加依赖检查（Go, pnpm）
  - 添加彩色日志输出
  - 简化构建配置数组，移除重复的构建目标
  - 添加 `go vet` 和 `golangci-lint` 检查步骤

- 更新 `scripts/dev.ps1` / `scripts/dev.sh`:
  - 改进进程管理和优雅关闭逻辑
  - 添加彩色日志输出
  - PowerShell 版本添加防抖（1秒）避免重复构建
  - 添加交互控制（Q停止，R手动重建）
  - 改进信号处理和清理逻辑

- 更新 `scripts/README.md`:
  - 添加新参数的文档说明
  - 添加依赖要求表格
  - 更新特性对比表格

- 更新 `.github/workflows/check.yml`:
  - 添加并发控制（避免重复运行）
  - 分离 lint 任务为独立 job
  - 添加多平台构建检查矩阵（linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64）
  - 添加 pnpm 缓存优化
  - 添加 race 检测 (`go test -race`)

- 更新 `.github/workflows/release.yml`:
  - 添加并发控制（发布任务串行执行）
  - 添加测试和 lint 检查步骤
  - 改进发布说明处理（支持默认说明）
  - 添加构建摘要输出到 GitHub Actions Summary

**验证:** `go build` ✅ `go test` ✅ `go vet` ✅ `golangci-lint` ✅ `前端构建` ✅

---

## 执行策略

每个 Phase 内的步骤按顺序执行，每步完成后：
1. `go build ./...` 确保编译通过
2. `go test ./...` 确保测试通过
3. `go vet ./...` 确保无静态问题
4. `golangci-lint run` 确保无 lint 问题
5. 前端需在浏览器中完整测试播放/列表/设置功能
6. 脚本验证：`./scripts/build.sh --skip-tests --skip-lint` 或 `.\scripts\build.ps1 -SkipTests -SkipLint`

如遇编译错误，立即修复再进入下一步。不跳步。

---

## Todo List

- [x] Phase 0: 准备工作
  - [x] 0a: 创建 domain 包，移动 Share 类型
  - [x] 0b: 移动 types.go 类型到 domain
  - [x] 0c: 更新 util 包使用 domain.Share
- [x] Phase 1: 拆分后端大文件
  - [x] 1a: 拆分 handler/handlers.go
  - [x] 1b: 拆分 media/scanner.go → scanner 包
  - [x] 1c: 从 transcoder.go 拆出 probe.go
  - [x] 1d: 提取 cache/media.go
- [x] Phase 2: 引入接口 + 依赖注入
  - [x] 2a: 创建 storage 包
  - [x] 2b: Handler 依赖接口
  - [x] 2c: 删除全局 db.DB
  - [x] 2d: 提取 service/session.go
  - [x] 2e: 添加 service/media.go
- [x] Phase 3: 前端解耦
  - [x] 3a: 创建 eventbus.js
  - [x] 3b: 断开 player↔playlist 循环
  - [x] 3c: 断开 ui↔actions 循环
  - [x] 3d: 拆分 player.js
  - [x] 3e: 拆分 playlist.js
  - [x] 3f: 拆分 ui.js
- [x] Phase 4: 收尾优化
  - [x] 4a: server.go 瘦身至 ~200行
  - [x] 4b: 添加优雅关机
  - [x] 4c: probe.go 添加 ffprobe 结果缓存
  - [x] 4d: 正则预编译
  - [x] 4e: main.go 重构为依赖组装器
- [x] 脚本和工作流更新
  - [x] 更新 build.ps1 / build.sh（添加参数、彩色日志、lint检查）
  - [x] 更新 dev.ps1 / dev.sh（优雅关闭、防抖、交互控制）
  - [x] 更新 scripts/README.md
  - [x] 更新 .github/workflows/check.yml（并发控制、矩阵构建）
  - [x] 更新 .github/workflows/release.yml（并发控制、构建摘要）
