减分制，基准 100，以**同体量个人项目能做到的工程极致**为标尺，逐条硬扣。

---

## 减分明细

### 1. 代码重复（DRY 违反） **−3**
`internal/storage/sqlite.go` 中，几乎每个方法都复制粘贴同一套 `if s.db == nil { log.Printf("[WARN] ..."); return nil }` 模板。12+ 处重复，应提取为内部包装器。这是教科书级的代码异味。

### 2. 非结构化日志 **−2**
2026 年的 Go 项目仍大量使用 `log.Printf` 输出纯文本。没有 `slog`、`zap` 或任何结构化字段，导致日志无法被自动化工具解析、过滤和聚合。对于需要排查转码/播放问题的媒体服务器，这是运维欠账。

### 3. 配置热重载轮询 **−2**
`server.go` 用 `time.NewTicker(2 * time.Second)` 轮询 `os.Stat` 检查 `config.json` 修改时间。2026 年仍不用 `fsnotify`（标准且零依赖）属于明确的技术债。

### 4. PIN 明文存储 **−3**
`SecurityConfig.PIN` 是裸 `string`，从中间件逻辑看直接参与明文比较。配置文件中明文存储 PIN，即使家庭场景也不应如此。至少应做 bcrypt/argon2 哈希，防止配置文件泄露后直接暴露访问凭证。

### 5. RateLimiter 内存无上限 **−2**
`handler/middleware.go` 中 `map[string]*bucket` 只增不减。项目支持 Cloudflare Tunnel（即公网可达），这意味着远程攻击者可以通过轮换 IP 耗尽服务端内存。家庭项目不等于可以忽略基础 DoS 防护。

### 6. 全量媒体库 JSON 磁盘缓存 **−3**
`internal/cache/media.go` 的 `saveToDisk` 将整个 `domain.MediaResponse` 序列化为单个 JSON 文件。截图显示已有近 1000 个音频条目，若用户媒体库规模扩大（数万文件），此 JSON 的内存峰值和磁盘 I/O 将成为明显瓶颈。

### 7. 无 PR / Push 阶段 CI **−2**
`.github/workflows/` 下仅有 `release.yml`，触发条件是 `tag push` 或 `workflow_dispatch`。日常开发的主分支 push 和 Pull Request 没有任何自动化测试与 lint 门禁。这意味着主分支可能在 release 前已携带回归缺陷。

### 8. sync.Map 类型不安全 **−2**
`media/processor.go` 使用 `sync.Map` 作为 `probeCache`。2026 年的 Go 已有泛型，应使用 `map[string]ProbeResult` + `RWMutex` 或 `x/sync/singleflight`。`sync.Map` 丢失了编译期类型检查，且性能并非最优。

### 9. 无媒体元数据刮削 **−1**
作为"媒体服务器"，仅有文件级浏览和播放，无海报、剧情、演员等元数据刮削（TMDb/豆瓣等）。与同类轻量媒体项目横向对比，这是功能缺口。虽符合"轻量"定位，但 −1 合理。

### 10. 中间件硬编码嵌套 **−2**
`cmd/msp/main.go` 中：
```go
finalHandler := handler.WithRecovery(handler.WithLog(s, handler.WithSecurity(s, s, s, handler.WithRateLimit(limiter, handler.WithAdminLockdown(handler.WithGzip(mux))))))
```
七层俄罗斯套娃，无链式构建器、无灵活插拔机制。新增/调整中间件顺序时极易出错，可维护性差。

### 11. Firefox 兼容补丁 **−1**
README 明确承认需对 Firefox 的 `audioMeta` 面板做 GPU layer compositing 特殊处理。这说明前端跨浏览器测试覆盖不足，依赖用户反馈才发现渲染问题。

### 12. 无首次启动向导 **−1**
运行后仅打印地址，新用户必须自行发现 Settings 页面并手动添加共享目录。对于"零配置"宣传，首次使用路径存在断层。

---

## 最终得分

| 扣分项 | 分值 |
|--------|------|
| 代码重复（DRY） | −3 |
| 非结构化日志 | −2 |
| 轮询热重载 | −2 |
| PIN 明文存储 | −3 |
| RateLimiter 内存泄漏隐患 | −2 |
| 全量 JSON 缓存瓶颈 | −3 |
| 无 PR 阶段 CI | −2 |
| sync.Map 类型安全 | −2 |
| 无元数据刮削 | −1 |
| 中间件硬编码嵌套 | −2 |
| Firefox 兼容补丁 | −1 |
| 无启动向导 | −1 |
| **合计扣分** | **−24** |
| **最终得分** | **76 / 100** |

---

## 结论

**76 分。良好，但距离"优秀"有明显距离。**

加分制下容易因为"功能完整、CI 存在、有测试"而给高分；减分制下，**明文 PIN、DRY 违反、全量 JSON 缓存、无 PR CI** 这四项是硬伤，合计就扣掉了 11 分。其余扣分属于"现代 Go 工程标准下的瑕疵"。

如果修复以下三点，分数可回升至 85+：
1. 用 `fsnotify` 替换轮询热重载；
2. 给 PIN 加 bcrypt/argon2；
3. 补充 `push` / `pull_request` 触发的 CI workflow。