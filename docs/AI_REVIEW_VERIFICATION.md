# MSP 项目外部 AI 评审核验报告

> 核验日期: 2026-05-16  
> 核验对象:
> - `docs/review_by_haiku_4_6.md`
> - `docs/review_by_sonnet_4_6.md`
> - 当前代码基线: HEAD (含未提交变更)

---

## 一、概述

本报告对两份外部 AI 评审中提出的每一个问题进行逐条代码级核验，给出**属实 / 部分属实 / 不属实**的判定，并附上代码位置、实际行为和影响评估。

| 来源 | 提出问题数 | 属实 | 部分属实 | 不属实 |
|------|-----------|------|----------|--------|
| review_by_haiku_4_6.md | 7 | 3 | 2 | 2 |
| review_by_sonnet_4_6.md | 6 | 5 | 1 | 0 |
| **合计** | **13** | **8** | **3** | **2** |

---

## 二、review_by_haiku_4_6.md 问题核验

### 问题 1: sqlite.go 多处存在"静默失败"（High优先级）

**AI 原述:**
> `sqlite.go:141-145` — `GetScanMeta` 中 `s.db == nil || cacheKey == ""` 时直接返回 `false, nil`，没有日志。

**核验结果:** ✅ **属实，且范围比 AI 指出的更大**

**证据位置:**
- `internal/storage/sqlite.go:92-94` (`GetProgress`)
- `internal/storage/sqlite.go:103-106` (`SetProgress`)
- `internal/storage/sqlite.go:117-119` (`GetAllPrefs`)
- `internal/storage/sqlite.go:132-134` (`SetPrefs`)
- `internal/storage/sqlite.go:152-155` (`GetScanMeta`) ← AI 指出的位置
- `internal/storage/sqlite.go:169-170` (`SetScanMeta`)
- `internal/storage/sqlite.go:183-184` (`UpsertMediaItem`)
- `internal/storage/sqlite.go:197-198` (`UpsertMediaItems`)
- `internal/storage/sqlite.go:211-212` (`DeleteStaleByScan`)
- `internal/storage/sqlite.go:222-224` (`DeleteByShareRootsNotIn`)
- `internal/storage/sqlite.go:232-234` (`QueryMediaItems`)
- `internal/storage/sqlite.go:244-246` (`CountMediaItems`)

**实际代码模式:**
```go
func (s *SQLite) GetScanMeta(ctx context.Context, cacheKey string) (domain.MediaScan, bool, error) {
    if s.db == nil || cacheKey == "" {
        return domain.MediaScan{}, false, nil  // 无任何日志，静默失败
    }
    // ...
}
```

**影响评估:** 如果数据库初始化失败（如权限问题），后续所有存储操作都会静默返回空值/零值，运维时难以定位根因。但项目目前仅在启动时初始化一次数据库，正常路径下 `s.db` 为 nil 的概率极低。

---

### 问题 2: cache/media.go 缓存预热"竞态条件"

**AI 原述:**
> `cache/media.go:95-100` — `if c.building` 时返回不完整数据，`r.Scanning = true` 可能无限期为 true，导致前端重复加载。

**核验结果:** ⚠️ **部分属实，但描述存在偏差**

**证据位置:** `internal/cache/media.go:96-101`

```go
if c.building {
    r := c.unmarshalResp()
    r.Scanning = true
    etag := c.etag
    c.mu.Unlock()
    return r, etag
}
```

**实际行为分析:**
1. `building` 被设置为 `true` 的场景有 3 处：
   - 第 85 行：TTL 过期触发后台 rebuild
   - 第 105 行：前端请求 `refresh=true`
   - 第 136 行：缓存 key 变化，需要全新构建
2. `building` 被重置为 `false` 仅在 `buildAndUpdate` 中（第 178-181 行失败路径、第 195-202 行成功路径），并通过 `c.cond.Broadcast()` 通知等待者。
3. **`building` 不会"无限期为 true"** —— 即使 `rebuild` 失败，`buildAndUpdate` 也会将其设为 `false`。

**真正的问题:** 返回旧数据+`Scanning=true` 是**设计行为**（快速响应+前端轮询），不是竞态条件 bug。但如果 `buildAndUpdate` panic（已被 `WithRecovery` 中间件捕获，但不在 MediaCache 内部），`building` 确实可能卡住。这是一个**低概率的防御性编程缺陷**，而非 AI 描述的"竞态条件"。

---

### 问题 3: handler/progress.go 客户端日志注入防护不完整

**AI 原述:**
> `progress.go:95-108` — 有 log level 白名单，但"消息内容限制呢？审计时提到 500 字符限制"。

**核验结果:** ❌ **不属实，防护已完整**

**证据位置:** `internal/handler/progress.go:84-106`

```go
func (h *Handler) HandleLog(w http.ResponseWriter, r *http.Request) {
    // ...
    if req.Level != "" && !validLogLevels[req.Level] {
        writeError(w, http.StatusBadRequest, "invalid log level")
        return
    }
    if req.Msg != "" {
        if len(req.Msg) > 500 {      // ← 500 字符限制存在
            req.Msg = req.Msg[:500]
        }
        req.Msg = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(req.Msg)
        h.logger.Log(req.Level, req.Msg)
    }
    w.WriteHeader(http.StatusNoContent)
}
```

**结论:** 代码中已经实现了：① level 白名单、② 500 字符截断、③ 换行符替换。AI 声称"消息内容限制缺失"是错误的，可能是阅读代码时遗漏了第 99 行。

---

### 问题 4: middleware.go IPv6 zone index 处理缺失

**AI 原述:**
> `middleware.go:272-280` — `getAccessLevelFromRequest` 里没有调用 zone index 处理。

**核验结果:** ❌ **不属实，处理链完整**

**证据位置:**
- `internal/handler/middleware.go:424-426` (`getAccessLevelFromRequest`)
- `internal/handler/middleware.go:272-280` (`getClientIP`)
- `internal/handler/middleware.go:376-380` (`getAccessLevel`)

**调用链:**
```
getAccessLevelFromRequest(r)
  → getClientIP(r, false)          // 提取 IP，可能含 zone index
  → getAccessLevel(clientIP)       // 内部处理 zone index
```

```go
func getAccessLevel(clientIP string) AccessLevel {
    // Strip IPv6 zone index (e.g. "fe80::1%eth0" -> "fe80::1")
    if idx := strings.Index(clientIP, "%"); idx != -1 {
        clientIP = clientIP[:idx]
    }
    // ...
}
```

**结论:** zone index 的剥离在 `getAccessLevel` 中完成，而 `getAccessLevelFromRequest` 最终会调用它。AI 可能将 `getClientIP` 和 `getAccessLevelFromRequest` 混为一谈，导致误判。

---

### 问题 5: config.go 原子操作不足

**AI 原述:**
> `config.go:99-108` — 结构体赋值是原子的，但 `s.cfg = cfg` 和 `s.cfgModTime = stat.ModTime()` 两步操作间有间隙。

**核验结果:** ❌ **不属实，赋值受锁保护**

**证据位置:** `internal/server/server.go:151-154`

```go
s.mu.Lock()
s.cfg = cfg
s.cfgModTime = stat.ModTime()
s.mu.Unlock()
```

**结论:** `cfg` 和 `cfgModTime` 的赋值都在同一把互斥锁的保护下完成，不存在"间隙"。AI 引用的行号（`config.go:99-108`）对应的是 `Config` 结构体定义，完全不涉及赋值逻辑，属于**定位错误**。

---

### 问题 6: 性能隐患（连接池/内存缓存/FFprobe）

**AI 原述:**
> 数据库连接池不可配、内存缓存大小无限制可能 OOM、FFprobe 每个未缓存文件都要执行。

**核验结果:** ⚠️ **部分属实**

| 子项 | 核验结果 | 说明 |
|------|----------|------|
| 连接池不可配 | ✅ 属实 | `sqlite.go:55-56` 硬编码 `SetMaxOpenConns(1)` / `SetMaxIdleConns(1)`，无配置入口 |
| 内存缓存无限制 | ⚠️ 夸大 | `probeCache` 是 `sync.Map`，但缓存的是 `CodecInfo`（两个字符串），条目数等于媒体文件数，正常家用场景下 OOM 风险极低 |
| FFprobe 调用 | ✅ 属实 | 未命中缓存时确实执行 `ffprobe`，但这是业务需求；已有 5 分钟 TTL 缓存 (`probeTTL`) |

---

### 问题 7: 测试覆盖不足

**AI 原述:**
> 有单元测试，但集成测试覆盖可能有限，缺少端到端测试。

**核验结果:** ✅ **属实**

**证据:**
- 运行 `go test -coverprofile=coverage.out ./...` 结果：总覆盖率 **62.6%**
- 各包覆盖率：
  - `cmd/msp`: 0.0%
  - `internal/cache`: 39.8%
  - `internal/config`: 88.6%
  - `internal/handler`: 57.4%
  - `internal/media`: 61.8%
  - `internal/scanner`: 82.3%
  - `internal/server`: 51.6%
  - `internal/storage`: 65.3%
- 项目中**不存在** `integration_test.go` 或 `e2e_test.go` 文件。

---

## 三、review_by_sonnet_4_6.md 问题核验

### Prompt 1: AES-GCM 随机 nonce 导致 MediaID 不稳定（最重要）

**AI 原述:**
> v1.5.0 引入 AES-GCM 加密生成媒体 ID，但使用随机 nonce，导致每次扫描同一文件产生不同 ID。v1.5.1 将 `UpsertMediaItems` 的 `OnConflict` 从 `id` 改为 `path` 是临时补丁。

**核验结果:** ✅ **完全属实，根因分析准确**

**证据位置 1 — ID 生成:** `internal/util/crypto_id.go:37-57`

```go
func EncodeID(path string) string {
    // ...
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {  // ← 随机 nonce
        // ...
    }
    ciphertext := gcm.Seal(nonce, nonce, []byte(path), nil)
    return base64.RawURLEncoding.EncodeToString(ciphertext)  // 每次输出不同
}
```

**证据位置 2 — 调用点:** `internal/scanner/scanner.go:189-190`

```go
item := domain.MediaItem{
    ID: util.EncodeID(path),  // 同一文件每次扫描 ID 不同
```

**证据位置 3 — 补丁痕迹:** `internal/storage/sqlite.go:200-203`

```go
func (s *SQLite) UpsertMediaItems(...) error {
    // ...
    return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
        Columns:   []clause.Column{{Name: "path"}},  // ← 按 path 冲突，非常规按主键
        UpdateAll: true,
    }).Create(&items).Error
}
```

**对比:** 同文件的 `UpsertMediaItem`（单数）仍使用 `Columns: []clause.Column{{Name: "id"}}`（`sqlite.go:187`），复数版本改为 `path`，证实了 Sonnet 的推断——这是为了兼容不稳定 ID 的权宜之计。

**副作用:**
- 书签/历史中的旧 ID 在重新扫描后失效
- 播放进度 (`PlaybackProgress`) 以 `media_id` 为外键，ID 变化后进度丢失
- 数据库中会积累孤儿记录（旧 ID 的 `MediaItem` 不会被清理）

---

### Prompt 2: go.mod 模块名不符合规范

**AI 原述:**
> `go.mod` 第一行是 `module msp`，应为 `module github.com/blycr/msp`。

**核验结果:** ✅ **属实，但严重程度有限**

**证据:** `go.mod:1`

```go
module msp
```

**实际影响:**
- 外部项目无法 `go get github.com/blycr/msp` 正确引用
- 但 MSP 是一个**独立应用**（single-binary media server），不是一个供导入的库，内部 import 都使用 `"msp/internal/..."`
- 修改模块名需要全库替换 import 路径，属于破坏性变更

**结论:** 技术上违反 Go 模块命名最佳实践，但对当前项目的实际运行无影响。如果未来计划将部分包作为库对外提供，则需要修复。

---

### Prompt 3: README 中错误引用 Gin 框架

**AI 原述:**
> README Acknowledgements 写了 Gin，但 go.mod 中没有 Gin 依赖，说明已迁移到标准库，文档未同步。

**核验结果:** ✅ **完全属实**

**证据 1 — README:** `README.md:105`

```markdown
* [Gin](https://github.com/gin-gonic/gin) - HTTP web framework written in Go.
```

**证据 2 — go.mod 依赖:**
- 直接依赖仅 3 个：`sqlite`, `testify`, `gorm`
- 无 `gin-gonic/gin`

**证据 3 — HTTP 实现:** `cmd/msp/main.go:143-173`

```go
mux := http.NewServeMux()  // 标准库
mux.Handle("/api/config", http.HandlerFunc(h.HandleConfig))
// ...
srv := &http.Server{Addr: addr, Handler: finalHandler}
```

**结论:** 项目完全基于 `net/http` 标准库，README 中的 Gin 引用是历史残留，需要删除。

---

### Prompt 4: Firefox audioMeta 黑块问题

**AI 原述:**
> README 记载 Firefox audioMeta 可能渲染为黑块，"建议换用 Chrome"不是修复。

**核验结果:** ⚠️ **部分属实——代码已修复，但 README 未更新**

**证据 1 — README 记载:** `README.md:34`

```markdown
> **Note for Firefox users:** The audio metadata panel (`audioMeta`) may occasionally render as a black block. For the best experience, a Chromium-based browser is recommended.
```

**证据 2 — CSS 修复:** `web/src/styles/other.css:32-37`

```css
/* Firefox 兼容性：避免 audioMeta 在 opacity transition 时出现黑块 */
@supports (-moz-appearance: none) {
  #audioMeta {
    transform: translateZ(0);
  }
}
```

**证据 3 — JS 修复:** `web/src/modules/player/play.js:245-247`

```js
if (typeof navigator !== "undefined" && navigator.userAgent.includes("Firefox")) {
    meta.style.transform = "translateZ(0)";
}
```

**结论:** 代码中已经针对 Firefox 做了专项处理（强制 GPU 层合成），并且用户已经验证修复完成，但 README 中的已知问题说明仍然存在。这说明：
- README 应该更新为"已做兼容性处理"或至少删除"推荐 Chrome"的表述。

---

### Prompt 5: 缺少测试覆盖率 Badge 和 CI 覆盖率上报

**AI 原述:**
> 项目没有测试覆盖率徽章，也没有在 CI 中上报覆盖率。

**核验结果:** ✅ **属实**

**证据 1 — CI 配置:** `.github/workflows/check.yml`
- `test` job 运行 `go test -v -race ./...`，无 `-coverprofile` 参数
- 无 `codecov` 或类似上报步骤

**证据 2 — README 徽章:** `README.md:7-11`
- 现有徽章：release、go version、license、repo size、DeepWiki
- 无覆盖率徽章

**证据 3 — 本地覆盖率:** 当前总覆盖率 **62.6%**

---

### Prompt 6: 缺少集成测试

**AI 原述:**
> 缺少"扫描 → 入库 → API 返回"完整链路的集成测试。

**核验结果:** ✅ **属实**

**证据:**
- 项目中无任何 `integration_test.go` 或 `e2e_test.go`
- `handler` 包中大量测试使用 mock 或 httptest 单独测试 handler，但未覆盖与 database、scanner、processor 的联合行为
- Sonnet 提出的 5 个高价值场景（`TestScanThenList`、`TestMediaIDStability`、`TestStreamRange`、`TestPINBruteForce`、`TestLANAccessControl`）均未实现

---

## 四、额外发现（未被两份评审覆盖）

在核验过程中，还发现了以下问题，两份 AI 评审均未提及：

### 4.1 `globalIDKey` 是 package-level 全局变量

**位置:** `internal/util/crypto_id.go:13-18`

```go
var globalIDKey []byte

func SetIDKey(key []byte) {
    globalIDKey = key
}
```

**问题:** Sonnet 在 Prompt 1 中明确要求"不允许使用 package-level 全局变量存储 key"，但当前的 `SetIDKey` 正是这样做的。虽然 Sonnet 把它归入"需要修改"的范畴，但在核验阶段这是值得记录的现有事实。

### 4.2 `UpsertMediaItems` 与 `UpsertMediaItem` 冲突策略不一致

**位置:** `internal/storage/sqlite.go:178-204`

- `UpsertMediaItem`（单数）: `OnConflict` 列是 `id`
- `UpsertMediaItems`（复数）: `OnConflict` 列是 `path`

**问题:** 同一文件内两个高度相关的函数使用不同的冲突策略，没有注释说明原因，对新开发者造成困惑。这是 v1.5.1 补丁的遗留痕迹。

### 4.3 `isValidIP` 仅支持 IPv4

**位置:** `internal/config/validate.go:225-241`

```go
func isValidIP(s string) bool {
    parts := strings.Split(s, ".")
    if len(parts) != 4 { return false }
    // ...
}
```

**问题:** 安全配置的 IP 白名单/黑名单验证函数无法验证 IPv6 地址，但中间件 `matchesCIDR` 和 `getAccessLevel` 都支持 IPv6。这意味着用户可以在配置中输入 IPv6 CIDR（如 `::1/128`）并通过 `isValidIPOrCIDR` 的 CIDR 分支验证，但纯 IPv6 地址（如 `::1`）会被 `isValidIP` 拒绝。这是一个边界情况缺陷。

### 4.4 `LoadFromDisk` 存在逻辑冗余

**位置:** `internal/cache/media.go:220-230`

```go
already := c.key == key && !c.builtAt.IsZero()
need := c.key != key || c.builtAt.IsZero()
if already || !need {  // already || !(c.key != key || c.builtAt.IsZero())
    return already
}
```

**问题:** `already || !need` 恒等于 `already || (c.key == key && !c.builtAt.IsZero())`，即 `already`，所以 `need` 变量的计算是冗余的。虽然不影响正确性，但属于不必要的复杂逻辑。

---

## 五、总结与优先级建议

### 5.1 已确认的真实问题（按优先级排序）

| 优先级 | 问题 | 来源 | 影响 |
|--------|------|------|------|
| P0 | **AES-GCM 随机 nonce 导致 MediaID 不稳定** | Sonnet #1 | 播放进度丢失、数据库孤儿记录、书签失效 |
| P1 | **README 错误引用 Gin** | Sonnet #3 | 误导贡献者 |
| P1 | **缺少覆盖率 Badge / CI 上报** | Sonnet #5 | 透明度低 |
| P2 | **多处数据库操作静默失败** | Haiku #1 | 运维困难，但触发概率低 |
| P2 | **缺少集成测试** | Sonnet #6 | 回归风险 |
| P3 | **go.mod 模块名不规范** | Sonnet #2 | 仅影响外部引用 |
| P3 | **README 未更新 Firefox 修复状态** | Sonnet #4 | 文档过时 |
| P3 | **SQLite 连接池硬编码** | Haiku #6 | 不可调优 |

### 5.2 AI 评审中的误判

| 问题 | 来源 | 误判原因 |
|------|------|----------|
| 日志注入"缺少 500 字符限制" | Haiku #3 | 漏看了 `progress.go:99` 的截断逻辑 |
| "IPv6 zone index 未处理" | Haiku #4 | 将 `getClientIP` 和 `getAccessLevel` 的调用链割裂分析 |
| "config 赋值非原子" | Haiku #5 | 行号定位错误（指向结构体定义而非赋值逻辑），且未看到锁保护 |

### 5.3 整体评价

两份 AI 评审的**核心高价值发现**（MediaID 不稳定、模块名、文档与代码不一致、缺少集成测试）均准确且有代码证据支撑。Haiku 的评审在细节上存在少量误判（行号错位、阅读遗漏），但其指出的"静默失败"问题确实存在且范围更广。

**最应优先处理的是 Sonnet #1（确定性 MediaID）**，它是数据一致性的根本问题，当前按 `path` 冲突的补丁只是掩盖症状。

---

*报告生成完毕。所有代码引用均基于 2026-05-16 的代码快照。*
