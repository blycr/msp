# AI Review 核验报告 — review_by_kimi.md

> 核验日期：2026-05-16  
> 基准代码：main @ 24d8f29 (v1.6.0+)  
> 方法：逐条定位源码，验证指控是否成立。

---

## 核验总览

| 条目 | 指控 | 结论 | 扣分是否应保留 |
|------|------|------|----------------|
| 1 | 代码重复（DRY 违反） | ✅ **属实** | 保留 |
| 2 | 非结构化日志 | ✅ **属实** | 保留 |
| 3 | 配置热重载轮询 | ✅ **属实** | 保留 |
| 4 | PIN 明文存储 | ⚠️ **核心属实，描述有偏差** | 保留（核心问题成立） |
| 5 | RateLimiter 内存无上限 | ✅ **属实** | 保留 |
| 6 | 全量 JSON 磁盘缓存 | ✅ **属实** | 保留 |
| 7 | 无 PR / Push 阶段 CI | ❌ **不属实** | **应移除** |
| 8 | sync.Map 类型不安全 | ✅ **属实** | 保留 |
| 9 | 无媒体元数据刮削 | ✅ **属实** | 保留 |
| 10 | 中间件硬编码嵌套 | ✅ **属实** | 保留 |
| 11 | Firefox 兼容补丁 | ✅ **属实** | 保留 |
| 12 | 无首次启动向导 | ✅ **属实** | 保留 |

**统计：10 属实 / 1 部分 / 1 不属实**

---

## 逐条详细核验

### 1. 代码重复（DRY 违反） −3 ✅ 属实

**源码位置**：`internal/storage/sqlite.go` 第 92–304 行

**证据**：以下 14 个公开方法中，每个都以完全相同的模板开头：

```go
if s.db == nil {
    log.Printf("[WARN] SQLite.XXX: database not initialized")
    return nil // 或对应零值
}
```

涉及方法：
`GetProgress` · `SetProgress` · `DeleteProgress` · `ListAllProgress` · `GetAllPrefs` · `SetPrefs` · `GetScanMeta` · `SetScanMeta` · `UpsertMediaItem` · `UpsertMediaItems` · `DeleteStaleByScan` · `DeleteByShareRootsNotIn` · `QueryMediaItems` · `CountMediaItems`

此外，4 个含 `tx *gorm.DB` 参数的方法（SetScanMeta、UpsertMediaItem、UpsertMediaItems、DeleteStaleByScan、DeleteByShareRootsNotIn）还重复了 `dbConn := s.db; if tx != nil { dbConn = tx }` 的模板。

**结论**：教科书级的 DRY 违反，review 指控成立。

---

### 2. 非结构化日志 −2 ✅ 属实

**源码位置**：全项目

**证据**：
- `cmd/msp/main.go`：第 43 行 `log.SetFlags(log.LstdFlags | log.Lmicroseconds)`，全局使用 `log.Printf`。
- `internal/storage/sqlite.go`：大量 `[WARN] SQLite.XXX: ...` 格式。
- `internal/cache/media.go`：第 63、118、123、142、217、248 行 `[WARN] ...`。
- `internal/handler/middleware.go`：第 31 行 `[PANIC] ...`。
- `internal/media/processor.go`：第 87 行 `[INFO] ...`。

**缺失**：无 `slog`、`zap`、`logrus` 或任何结构化字段（key-value 对、JSON 输出、日志级别过滤）。

**结论**：review 指控成立。

---

### 3. 配置热重载轮询 −2 ✅ 属实

**源码位置**：
- `internal/config/config.go` 第 14 行：`DefaultConfigCheckInterval = 2 * time.Second`
- `internal/server/server.go` 第 109–121 行：

```go
func (s *Server) WatchConfig(ctx context.Context) {
    ticker := time.NewTicker(config.DefaultConfigCheckInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            s.checkAndReloadConfig()
        }
    }
}
```

第 123–158 行 `checkAndReloadConfig()` 使用 `os.Stat` 比较 `ModTime()`。

**结论**：确实使用 `time.NewTicker(2s)` + `os.Stat` 轮询，未使用 `fsnotify`（标准库 `golang.org/x/sys/windows` 或第三方 `github.com/fsnotify/fsnotify`）。review 指控成立。

---

### 4. PIN 明文存储 −3 ⚠️ 核心属实，描述有偏差

**源码位置**：
- `internal/config/config.go` 第 95 行：`PIN string json:"pin"`
- `internal/handler/auth.go` 第 90 行：`valid := constantTimeCompare(req.PIN, cfg.Security.PIN)`

**核实**：
1. **明文存储 ✅ 属实**：`config.json` 中 `security.pin` 字段直接存储明文字符串（如 `"0000"`）。配置文件泄露即暴露凭证。
2. **"中间件直接参与明文比较" ⚠️ 描述有偏差**：
   - 中间件 `WithSecurity`（`middleware.go:94`）**并不直接比较 PIN**，它只检查 `session.ValidateSession(sessionToken)`。
   - 明文 PIN 比较发生在 `HandlePIN` 处理器中（`auth.go:90`），使用了 `crypto/subtle.ConstantTimeCompare`（时序攻击防护已做）。

**修正指控**："PIN 明文存储"核心问题成立；但说"中间件逻辑直接参与明文比较"不够精确，实际是 Handler 层在做明文比较，且已加了时序安全比较。

---

### 5. RateLimiter 内存无上限 −2 ✅ 属实

**源码位置**：`internal/handler/middleware.go` 第 181–218 行

**证据**：

```go
type RateLimiter struct {
    mu      sync.Mutex
    buckets map[string]*bucket
}

func (rl *RateLimiter) Allow(ip string, rate float64, capacity float64) bool {
    // ...
    b, exists := rl.buckets[ip]
    if !exists {
        b = &bucket{tokens: capacity, lastUpdate: time.Now()}
        rl.buckets[ip] = b  // ← 只增不减
    }
    // ...
}
```

**缺失**：无定期清理、无 LRU、无最大容量限制。

**攻击面**：项目支持 Cloudflare Tunnel（公网可达），攻击者可轮换 IP 或 IPv6 地址段，使 `buckets` map 无限增长。

**结论**：review 指控成立。

---

### 6. 全量媒体库 JSON 磁盘缓存 −3 ✅ 属实

**源码位置**：`internal/cache/media.go` 第 256–272 行

**证据**：

```go
func (c *MediaCache) saveToDisk(key string, builtAt time.Time, etag string, resp domain.MediaResponse) {
    v := mediaCacheOnDisk{
        Key:     key,
        BuiltAt: builtAt.UnixNano(),
        ETag:    etag,
        Resp:    resp,  // ← 整个 MediaResponse
    }
    b, err := json.Marshal(v)  // ← 全量序列化
    // ...
}
```

`mediaCacheOnDisk` 结构体将整个 `domain.MediaResponse` 嵌入单一 JSON。截图显示已有近 1000 个条目，若扩展至数万文件，JSON 文件将达数十 MB。

**缺失**：无分页、无流式序列化、无增量更新。

**结论**：review 指控成立。

---

### 7. 无 PR / Push 阶段 CI −2 ❌ **不属实**

**源码位置**：`.github/workflows/check.yml`

**证据**：该文件已存在（最早提交 `7799215`），包含：

```yaml
on:
  push:
    branches: ["**"]
  pull_request:

jobs:
  test:
    # go test -v -race -coverprofile=coverage.out ./...
  lint:
    # golangci-lint-action@v7
  build-check:
    # 矩阵构建 8 个平台
```

`.github/workflows/` 目录下实际有 **两个文件**：`check.yml` 和 `release.yml`，而非 review 所称的"仅有 release.yml"。

**结论**：review 在此项上存在**事实错误**，CI 早已覆盖 push 和 pull_request。

---

### 8. sync.Map 类型不安全 −2 ✅ 属实

**源码位置**：`internal/media/processor.go` 第 27 行

**证据**：

```go
type MediaProcessor struct {
    // ...
    probeCache sync.Map  // ← 无泛型类型参数
    // ...
}
```

`sync.Map` 的 `Load`/`Store` 使用 `any`（interface{}），编译期不检查类型。Go 1.18+ 应使用 `map[string]ProbeResult` + `sync.RWMutex` 或 `golang.org/x/sync/singleflight`。

**结论**：review 指控成立。

---

### 9. 无媒体元数据刮削 −1 ✅ 属实

**功能层面**：项目仅有文件级浏览（路径、文件名、大小、时长），无海报、剧情简介、演员表等元数据刮削（TMDb、豆瓣、TVDB 等）。

**结论**：review 指控成立。属于功能缺口而非工程缺陷。

---

### 10. 中间件硬编码嵌套 −2 ✅ 属实

**源码位置**：`cmd/msp/main.go` 第 101 行

**证据**：

```go
finalHandler := handler.WithRecovery(
    handler.WithLog(s,
        handler.WithSecurity(s, s, s,
            handler.WithRateLimit(limiter,
                handler.WithAdminLockdown(
                    handler.WithGzip(mux))))))
```

七层俄罗斯套娃，无中间件链式构建器（如 `alice.New().Append().Then()` 或自定义 `MiddlewareChain`）。调整顺序极易出错。

**结论**：review 指控成立。

---

### 11. Firefox 兼容补丁 −1 ✅ 属实

**源码位置**：`README.md` 第 35 行

**证据**：

```markdown
> **Note for Firefox users:** Compatibility treatments (GPU layer compositing)
> have been applied for the audio metadata panel (`audioMeta`).
> If rendering issues persist, a Chromium-based browser is recommended.
```

**结论**：review 指控成立。属于已知兼容性问题，已在前端做了针对性处理。

---

### 12. 无首次启动向导 −1 ✅ 属实

**源码位置**：`cmd/msp/main.go` 第 182–196 行 `printStartupBanner`

**证据**：首次启动仅打印配置路径和访问地址：

```go
log.Println("配置文件:", cfgPath)
fmt.Println("访问:", "http://...")
```

无引导用户添加共享目录、无 PIN 设置提示、无"快速开始"页面。

**结论**：review 指控成立。

---

## 修正后评分

| 项目 | 原扣分 | 修正 |
|------|--------|------|
| 代码重复（DRY） | −3 | 保留 |
| 非结构化日志 | −2 | 保留 |
| 轮询热重载 | −2 | 保留 |
| PIN 明文存储 | −3 | 保留（核心成立） |
| RateLimiter 内存泄漏隐患 | −2 | 保留 |
| 全量 JSON 缓存瓶颈 | −3 | 保留 |
| ~~无 PR 阶段 CI~~ | ~~−2~~ | **移除（不属实）** |
| sync.Map 类型安全 | −2 | 保留 |
| 无元数据刮削 | −1 | 保留 |
| 中间件硬编码嵌套 | −2 | 保留 |
| Firefox 兼容补丁 | −1 | 保留 |
| 无启动向导 | −1 | 保留 |
| **修正后合计** | | **−22** |
| **修正后得分** | | **78 / 100** |

---

## 结论

review_by_kimi.md 整体质量较高，11/12 条指控（含 1 条部分偏差）有代码支撑。唯 **第 7 条（无 PR CI）存在事实错误** —— `check.yml` 早已存在并覆盖 push 与 pull_request。

修正后得分从 **76 → 78**，评价仍为"良好，距离优秀有明显距离"。
