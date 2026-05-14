# Phase 3: MediaProcessor 重构计划

> **状态**：待执行 | **优先级**：P1 | **风险**：高（接口变更面大）
>
> 本文档在上下文压缩后作为独立参考，执行前请先通读全文。

---

## 1. 问题定义

`internal/media/` 包中大量使用**包级全局变量**隐藏依赖关系，导致：
- 无法并行测试（全局状态冲突）
- `SetTranscodeLimit` 运行时替换 channel 非并发安全
- `cacheTTL`（`time.Duration`）直接跨 goroutine 读写无同步
- 包级 `sync.Once` 重置（`ResetPathsForTest`）非并发安全

---

## 2. 全局变量清单

| 变量 | 文件 | 用途 | 当前注入方式 |
|------|------|------|-------------|
| `mediaDB` | `store.go:18` | SQLite 数据库连接 | `SetDB(sq)` |
| `ffmpegPath`, `ffprobePath`, `pathOnce` | `probe.go:44-48` | FFmpeg 路径发现 | 包级 `sync.Once` |
| `probeCache`, `cacheTTL` | `probe.go:28-30` | ffprobe 结果缓存 | `SetProbeCacheTTL` 直接赋值 |
| `transcodeLimit`, `activeProcesses` | `transcoder.go:17-23` | 并发限制和进程跟踪 | `SetTranscodeLimit` 直接替换 channel |
| `hwOnce`, `hwResult`, `hwDisabled` | `hwaccel.go:114-121` | 硬件加速检测 | `DetectHWAccel` 包级 `sync.Once` |

---

## 3. 目标架构

### 3.1 核心结构体

在 `internal/media/processor.go` 中定义：

```go
package media

type MediaProcessor struct {
    db *storage.SQLite

    probePaths struct {
        ffmpeg  string
        ffprobe string
        once    sync.Once
    }

    probeCache sync.Map
    probeTTL   atomic.Int64 // nanoseconds

    transcode struct {
        limit  chan struct{}
        active map[*exec.Cmd]struct{}
        mu     sync.Mutex
    }

    hwAccel struct {
        once     sync.Once
        result   *HWAccelResult
        disabled atomic.Bool
    }
}

func NewMediaProcessor(db *storage.SQLite) *MediaProcessor {
    mp := &MediaProcessor{db: db}
    mp.probeTTL.Store(int64(5 * time.Minute))
    mp.transcode.limit = make(chan struct{}, 2)
    mp.transcode.active = make(map[*exec.Cmd]struct{})
    return mp
}
```

### 3.2 构造函数选项模式（可选扩展）

如果未来需要自定义并发限制：

```go
type Option func(*MediaProcessor)

func WithTranscodeLimit(n int) Option {
    return func(mp *MediaProcessor) {
        if n > 0 {
            mp.transcode.limit = make(chan struct{}, n)
        }
    }
}

func NewMediaProcessor(db *storage.SQLite, opts ...Option) *MediaProcessor {
    mp := &MediaProcessor{db: db}
    mp.probeTTL.Store(int64(5 * time.Minute))
    mp.transcode.limit = make(chan struct{}, 2)
    mp.transcode.active = make(map[*exec.Cmd]struct{})
    for _, opt := range opts {
        opt(mp)
    }
    return mp
}
```

---

## 4. 迁移步骤（必须按顺序执行）

### Step 1: 创建 `internal/media/processor.go`

- 定义 `MediaProcessor` 结构体
- 定义 `NewMediaProcessor`
- 将 `IsDBAvailable()` 改为 `MediaProcessor.IsDBAvailable()` 方法

### Step 2: 迁移 `store.go`

- 删除全局变量 `mediaDB` 和 `SetDB()`
- 将 `LoadMediaFromDB`、`LoadMediaResponseFromDBScan`、`IndexMediaToDB`、`performScan` 等改为 `MediaProcessor` 的方法
- `newMediaResponse` 提取公共函数（`BuildMediaResponse` 和 `LoadMediaResponseFromDBScan` 中有重复构造逻辑）

### Step 3: 迁移 `probe.go`

- 删除全局 `probeCache`、`cacheTTL`、`ffmpegPath`、`ffprobePath`、`pathOnce`
- `resolveFFmpegPaths`、`probeCodecInfo`、`GetProbeCache`、`SetProbeCacheTTL`、`ClearProbeCache` 改为方法
- `cacheTTL` 改用 `atomic.Int64` 存储纳秒值
- 删除 `ResetPathsForTest`（测试时直接 `NewMediaProcessor(nil)` 即可）

### Step 4: 迁移 `transcoder.go`

- 删除全局 `transcodeLimit`、`activeProcesses`、`activeProcessesMu`
- `SetTranscodeLimit`、`TranscodeStream`、`KillAllTranscodeProcesses` 改为方法
- `transcodeLimit` 只允许在构造函数中设置一次（或通过 Option），禁止运行时替换。如果必须支持热重载，使用 `atomic.Pointer[chan struct{}]` 或读写锁保护

### Step 5: 迁移 `hwaccel.go`

- 删除全局 `hwOnce`、`hwResult`、`hwDisabled`、`hwMu`
- `DetectHWAccel`、`GetHWAccel`、`DisableHWAccel` 改为方法
- `hwDisabled` 改用 `atomic.Bool`
- 删除 `ResetHWAccelForTest`（如有）

### Step 6: 更新 `media.go`

- `BuildMediaResponse` 保持为包级函数（它不依赖全局状态）
- 提取 `newMediaResponse(shares []domain.Share) domain.MediaResponse` 消除 `BuildMediaResponse` 和 `LoadMediaResponseFromDBScan` 中的重复构造逻辑

### Step 7: 更新调用方

#### `cmd/msp/main.go`

```go
// 当前
media.SetDB(store.SQLite())
media.SetTranscodeLimit(...)

// 改为
processor := media.NewMediaProcessor(store.SQLite(), media.WithTranscodeLimit(...))
```

#### `internal/server/server.go`

- `Server` 结构体新增 `processor *media.MediaProcessor` 字段
- `New` 或初始化方法中创建 `MediaProcessor`
- 删除 `SetupLogger` 中对 `media.SetTranscodeLimit` 的调用

#### `internal/handler/` 接口调整

当前 `Handler` 结构体通过 `media.CheckFFmpeg()`、`media.TranscodeStream()` 等全局函数调用。改为：

```go
type Handler struct {
    // ... 现有字段 ...
    processor *media.MediaProcessor
}
```

`handler.go` 中的 `Deps` 结构体增加 `Processor *media.MediaProcessor`：

```go
type Deps struct {
    Config   ConfigProvider
    Media    MediaCacheProvider
    Session  SessionProvider
    Logger   Logger
    Progress ProgressStore
    Prefs    PrefsStore
    Processor *media.MediaProcessor // 新增
}
```

然后更新 `New` 函数：
```go
func New(deps Deps) *Handler {
    return &Handler{
        // ...
        processor: deps.Processor,
    }
}
```

涉及的 handler 方法：
- `HandleStream` → `h.processor.TranscodeStream`
- `HandleProbe` → `h.processor.FFmpegAvailable` / `CheckFFprobe`
- `HandleConfig`（`ConfigView` 中的 `FFmpegAvailable` / `FFprobeAvailable`）→ 通过 `processor` 查询

#### `internal/cache/media.go`

- `MediaCache` 结构体新增 `processor *media.MediaProcessor`
- `NewMediaCache` 签名改为 `NewMediaCache(processor *media.MediaProcessor, cacheFilePath string, ttl time.Duration)`
- `buildAndUpdate` 中调用 `h.processor.ReindexAndLoadMedia` 替代 `media.ReindexAndLoadMedia`
- `LoadFromDisk` 中调用 `h.processor.IsDBAvailable()` 替代 `media.IsDBAvailable()`

#### `internal/service/config.go`

- `ConfigView` 中的 `FFmpegAvailable` / `FFprobeAvailable` 需要通过 `processor` 获取
- `ConfigService` 结构体新增 `processor *media.MediaProcessor`

---

## 5. 依赖注入链路

```
cmd/msp/main.go
  └── server.New()
        ├── media.NewMediaProcessor(store.SQLite())  ← 创建 Processor
        ├── handler.New(Deps{Processor: processor})  ← 注入 Handler
        └── cache.NewMediaCache(processor, ...)      ← 注入 Cache
```

---

## 6. 测试策略

重构前必须确认：
```bash
go test -race ./...   # 当前无 race
```

重构后验证：
1. `go test ./internal/media/...` — 所有现有测试通过（使用 `NewMediaProcessor` 替代全局函数）
2. `go test ./...` — 全量测试无回归
3. `go vet ./...` — 无警告
4. 手动验证：启动应用，播放视频/音频，确认转码、探针、硬件加速检测正常

---

## 7. 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| 接口变更导致 handler 调用遗漏 | 编译器会捕获未更新的调用点（方法 vs 包函数签名不同） |
| `MediaCache` 构造签名变更影响所有测试 | 搜索所有 `cache.NewMediaCache` 调用点，统一更新 |
| `transcodeLimit` 从全局 channel 改为实例 channel，并发行为变化 | 保持相同的初始容量（2），并通过 `-race` 测试验证 |
| 热重载 `logLevel` 代码路径不受影响，但 `transcodeLimit` 不再支持运行时修改 | 当前只在 `main.go` 启动时设置一次，无运行时修改需求 |

---

## 8. 文件变更清单（预估）

| 文件 | 变更类型 |
|------|---------|
| `internal/media/processor.go` | 新增 |
| `internal/media/store.go` | 重构（删除全局变量） |
| `internal/media/probe.go` | 重构（删除全局变量） |
| `internal/media/transcoder.go` | 重构（删除全局变量） |
| `internal/media/hwaccel.go` | 重构（删除全局变量） |
| `internal/media/media.go` | 提取公共函数 |
| `cmd/msp/main.go` | 创建 Processor 并注入 |
| `internal/server/server.go` | 持有 Processor，传给 handler/cache |
| `internal/handler/handler.go` | Deps 增加 Processor 字段 |
| `internal/handler/stream.go` | 改用 h.processor |
| `internal/handler/probe.go` | 改用 h.processor |
| `internal/handler/config.go` | 改用 h.processor（FFmpeg 可用性查询） |
| `internal/cache/media.go` | 构造函数增加 processor 参数 |
| `internal/service/config.go` | 增加 processor 字段 |
| `internal/media/*_test.go` | 更新为使用 NewMediaProcessor |
| `internal/cache/*_test.go` | 更新 NewMediaCache 调用 |
| `internal/handler/*_test.go` | 更新 handler 构造 |

---

## 9. 回滚策略

本阶段改动面大，建议单独分支执行：
```bash
git checkout -b refactor/media-processor
# ... 完成所有修改并测试 ...
git checkout main
git merge refactor/media-processor
```

若发现问题，可直接 revert merge commit。
