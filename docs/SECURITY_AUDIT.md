# MSP 安全审计报告

> 审计日期：2026-05-15
> 审计范围：后端 Go 服务 + 前端 PWA
> 暴露面：Cloudflare Tunnel (`msp.blycr.xyz`) → 本地 `localhost:8099`
> 网络：双层 NAT，Windows 防火墙仅允许 `127.0.0.1/32, 192.168.31.0/24`

---

## 1. 已实现的防护措施（✅ 确认有效）

| 措施 | 实现位置 | 说明 |
|------|---------|------|
| 三层访问分级 | `internal/handler/middleware.go` | `Local` / `LAN` / `Remote`，通过 `RemoteAddr` + `CF-Connecting-IP` 判定 |
| 管理 API 拦截 | `internal/handler/middleware.go` | `WithAdminLockdown`：非 Local 的 POST `/api/config`、`/api/shares`、`/api/prefs` → 403 |
| 后端字段过滤 | `internal/handler/config.go` | Remote 时清空 `urls`、`lanIPs`、`blacklist`、`shares`、`port`、`logLevel`、`logFile`、`maxItems`、`security.ipWhitelist/ipBlacklist` |
| PIN 认证 | `internal/handler/auth.go` | 可选 4 位 PIN，`constantTimeCompare` 防时序攻击 |
| Session 管理 | `internal/service/session.go` | `crypto/rand` 生成 32 字节 token，7 天过期，内存存储 |
| Cookie 安全 | `internal/handler/auth.go` | `HttpOnly` + `SameSite=Lax` + `Secure` 动态（HTTPS 时） |
| 路径遍历防护 | `internal/util/util.go` | `DecodeID` → `NormalizePath` → `IsAllowedFile` → `EvalSymlinks` → `WithinRoot` |
| Stream 验证 | `internal/handler/stream.go` | `resolveMediaTarget` 双重验证 ID 解码 + 文件是否在 shares 目录内 |
| 转码安全 | `internal/media/transcoder.go` | 并发 semaphore 限制；`format` 白名单（mp4/mp3/aac/webm/ogg）；`bitrate` 仅允许数字+k/m；其余参数硬编码 |
| 安全响应头 | `internal/handler/middleware.go` | `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`X-XSS-Protection: 1; mode=block`、`Referrer-Policy: strict-origin-when-cross-origin` |
| 错误处理 | `internal/handler/middleware.go` | `WithRecovery`：panic 不返回堆栈给客户端，仅记录本地日志 |
| 请求限制 | `internal/handler/handler.go` | JSON body 限制 1MB；HTTP ReadHeader 10s / Read 15s / Idle 60s |
| 防火墙 | Windows 本地 | 入站规则仅允许 `127.0.0.1/32` + `192.168.31.0/24` |
| Cloudflare Access | Cloudflare 层 | OTP 身份验证，阻止未授权访问 Tunnel |

---

## 修复状态（2026-05-16）

| 发现 | 状态 | 文件 | 说明 |
|------|------|------|------|
| 2.1 PIN 暴力破解 | ✅ 已修复 | `internal/handler/auth.go` | 5 次错误后锁定 15 分钟 |
| 2.2 全局无速率限制 | ✅ 已修复 | `internal/handler/middleware.go` | Token-bucket 限流器，per-IP，Local 豁免 |
| 2.3 TOCTOU 竞态 | ✅ 已修复 | `internal/handler/stream.go` | Open 后二次验证 EvalSymlinks + IsAllowedFile |
| 2.4 Inline XSS | ✅ 已修复 | `internal/handler/stream.go` | 非媒体文件强制 `Content-Disposition: attachment` |
| 2.5 Refresh DoS | ✅ 已修复 | `internal/handler/media.go` | 全局 30 秒冷却 |
| 2.6 无 CSP | ✅ 已修复 | `internal/handler/middleware.go` | CSP 头已添加 |
| 2.7 LAN 暴露信息 | ✅ 已修复 | `internal/handler/config.go` | LAN 时过滤 port/log/shares.path |
| 2.8 PIN 时序侧信道 | ✅ 已修复 | `internal/handler/auth.go` | `crypto/subtle.ConstantTimeCompare` |
| 2.9 客户端日志注入 | ✅ 已修复 | `internal/handler/progress.go` | level 白名单 + msg 截断/过滤 |
| 2.10 Base64 ID 泄露路径 | ✅ 已修复 | `internal/util/crypto_id.go` | AES-GCM 加密，key 存 `msp.key` |
| 2.11 弱默认 PIN | ✅ 已修复 | `internal/constants/constants.go` | `DefaultPIN = ""`，空 PIN 拒绝验证 |
| 2.12 IP 黑白名单 CF 失效 | ✅ 已修复 | `internal/handler/middleware.go` | 回环+CF 头时用真实 IP 评估黑白名单 |
| 2.13 缺少 WriteTimeout | ✅ 已修复 | `cmd/msp/main.go` | `WriteTimeout: 60s` |
| 2.14 无 HSTS | ✅ 已修复 | `internal/handler/middleware.go` | HTTPS 请求时添加 HSTS 头 |
| X-XSS-Protection 废弃 | ✅ 已修复 | `internal/handler/middleware.go` | 头已移除 |
| GitHub 链接泄露仓库 | ✅ 已修复 | `web/index.html` | 链接已移除 |

**注意**：Base64 ID 加密后，数据库中缓存的旧媒体项仍保留旧 base64 ID。如需完全生效，可删除 `media_items` 表让系统重新扫描（会保留播放进度等其他数据）。

---

## 2. 发现的风险

### 🔴 Critical

#### 2.1 PIN 暴力破解 — 无速率限制

- **位置**：`internal/handler/auth.go` `HandlePIN`
- **现状**：没有任何 per-IP 或全局速率限制
- **攻击**：4 位 PIN 仅 10,000 种组合，攻击者可用脚本在数分钟内穷举
- **影响**：一旦 PIN 被破解，攻击者获得与 LAN 相同的权限（可看可播放，不能改配置）
- **修复建议**：
  - IP 级失败计数器（内存 map），5 次错误后冷却 15 分钟
  - 或引入延迟响应（每次失败增加 500ms 延迟，上限 5s）
  - 取消默认 PIN `"0000"`（见 2.11），强制用户设置非空 PIN

#### 2.2 全局无速率限制 — 全端点暴露

- **位置**：`internal/handler/middleware.go` 中间件链
- **现状**：没有任何端点有速率限制或请求节流
- **攻击**：攻击者可同时暴力破解 PIN、持续触发 `/api/media?refresh=1`、占满转码槽
- **修复建议**：在中间件层添加 per-IP token-bucket 限流器，至少保护 `/api/pin`、`/api/stream`、`/api/media?refresh=1`

### 🟠 High

#### 2.3 TOCTOU 竞态条件 → 路径遍历

- **位置**：`internal/handler/stream.go` `resolveMediaTarget`
- **现状**：先 `IsAllowedFile(target)` 检查路径，再 `os.Open(target)` 打开文件。检查与打开之间存在时间窗口
- **攻击**：在多用户共享目录、NFS 挂载、或其他进程可写目录的场景下，攻击者在 `IsAllowedFile` 检查后将合法文件替换为指向敏感路径（如 `C:\Windows\System32\config\SAM`）的符号链接，服务器随后打开并返回该文件
- **修复建议**：打开文件后再验证 fd 的路径。Windows 下可用 `syscall.GetFinalPathNameByHandle`，或至少对 `os.Open` 后的结果再次执行 `EvalSymlinks` + `IsAllowedFile`

#### 2.4 Stored XSS / 同域脚本执行 — inline 文件渲染

- **位置**：`internal/handler/stream.go` `serveDirect`
- **现状**：`Content-Disposition: inline`，`Content-Type` 从文件扩展名推导。如果 shares 目录中有 `.html`、`.svg`、`.js` 文件，浏览器会以原生 MIME 类型在同源下渲染
- **攻击**：攻击者上传 `evil.html` 到 share，诱骗已登录用户访问 `/api/stream?id=<evil.html>`。由于同源，恶意页面可以：
  - `fetch('/api/media')` 读取完整媒体目录
  - `fetch('/api/config')` 读取配置（虽然后端过滤了部分字段，但 Local/LAN 访问时仍能看到完整数据）
  - 外泄 `msp_session` cookie（虽然 HttpOnly 阻止 JS 读取，但恶意页面可以以用户身份发起同源请求）
- **修复建议**：非媒体白名单文件（视频/音频/图片/字幕）强制 `Content-Disposition: attachment`；或在 stream 响应上加 `Content-Security-Policy: default-src 'none'`

#### 2.5 DoS — 无限制的缓存刷新

- **位置**：`internal/handler/media.go` `HandleMedia`
- **现状**：`/api/media?refresh=1` 触发完整文件系统扫描重建缓存，无任何速率限制或冷却
- **攻击**：`while true; do curl .../api/media?refresh=1; done` 可持续占用 CPU 和磁盘 I/O
- **修复建议**：添加全局或 per-IP 刷新冷却（如 30 秒），或要求携带签名 nonce

#### 2.6 无 CSP 头 — XSS 放大器

- **位置**：`internal/handler/middleware.go` `applySecurityHeaders`
- **现状**：缺少 `Content-Security-Policy`
- **风险**：如果前端未来出现 XSS（如文件名未转义渲染），攻击者可窃取 cookie、外泄数据、以用户身份调用 API
- **修复建议**：
  ```
  Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' blob:; media-src 'self' blob:
  ```

#### 2.7 LAN 访问暴露更多信息

- **位置**：`internal/handler/config.go` GET 分支
- **现状**：`AccessLAN` 时 `shares.path`、`port`、`urls` 完整返回
- **攻击场景**：WiFi 密码被破解、访客网络隔离不当、局域网内有恶意设备（IoT/手机病毒）
- **影响**：攻击者能获取文件系统结构（`D:\Music`）、服务端口、局域网拓扑
- **修复建议**：LAN 也过滤 `shares.path`（清空或只留 label）、`port` 清空

### 🟡 Medium

#### 2.8 PIN 长度时序侧信道

- **位置**：`internal/handler/auth.go` `constantTimeCompare`
- **现状**：自定义实现中 `if len(a) != len(b) { return false }` 在长度不匹配时立即返回，跳过后续循环
- **攻击**：攻击者测量 `/api/pin` 响应时间，长度错误的 PIN 比长度正确的快约 50ns，可推导出 PIN 长度
- **修复建议**：替换为 `crypto/subtle.ConstantTimeCompare`，它在任意长度下都是常数时间

#### 2.9 客户端日志注入

- **位置**：`internal/handler/progress.go` `HandleLog`
- **现状**：`req.Level` 和 `req.Msg` 直接传给 `h.logger.Log(req.Level, req.Msg)`，无验证、无过滤、无速率限制
- **攻击**：伪造日志条目、注入换行符干扰日志解析、调用 `/api/log`  flooding 填满磁盘
- **修复建议**：`level` 白名单（`debug/info/warn/error`），`msg` 截断并过滤换行符，添加速率限制

#### 2.10 Base64 ID 泄露绝对路径

- **位置**：`internal/util/util.go` `EncodeID` / `DecodeID`
- **现状**：ID 是对绝对路径的简单 `base64.RawURLEncoding`，不是加密或 keyed hash
- **攻击**：任何人可 Base64 解码 ID 获取服务器精确目录结构（如 `C:\Users\Admin\Private\...`）
- **修复建议**：用 HMAC-SHA256 或数据库自增 ID 映射，使 URL 永不暴露物理路径

#### 2.11 弱默认 PIN `"0000"`

- **位置**：`internal/config/config.go` + `internal/constants/constants.go`
- **现状**：`DefaultPIN = "0000"`，`ApplyDefaults` 在 PIN 为空时自动设为 `"0000"`
- **攻击**：用户启用 `pinEnabled` 但忘记改 PIN，服务器被 trivially guessable 的 4 位码保护
- **修复建议**：取消默认 PIN。若 PIN 启用但未设置，拒绝启动或强制 UI 设置

#### 2.12 IP 黑白名单在 Cloudflare Tunnel 后失效

- **位置**：`internal/handler/middleware.go` `WithSecurity`
- **现状**：`getClientIP(r, false)` 只读 `RemoteAddr`。Tunnel 流量 `RemoteAddr = 127.0.0.1`（cloudflared 守护进程）
- **影响**：IP 黑白名单对远程用户完全无效——黑名单中的 IP 仍可通过 Tunnel 访问，白名单若排除 `127.0.0.1` 则意外阻止所有 Tunnel 流量
- **修复建议**：当 `RemoteAddr` 是回环且存在 `CF-Connecting-IP` 时，用该头中的真实 IP 评估黑白名单

#### 2.13 缺少 WriteTimeout

- **位置**：`cmd/msp/main.go` `http.Server`
- **现状**：配置了 `ReadHeaderTimeout`、`ReadTimeout`、`IdleTimeout`，但无 `WriteTimeout`
- **攻击**：攻击者打开 `/api/stream` 或转码连接后极慢读取（或不读取），goroutine 和 TCP socket 长期占用
- **修复建议**：添加 `WriteTimeout: 60 * time.Second`（流式传输可单独用超时处理器）

#### 2.14 无 HSTS 头

- **位置**：`internal/handler/middleware.go` `applySecurityHeaders`
- **现状**：无 `Strict-Transport-Security`
- **风险**：不可信网络中存在 SSL 降级攻击面（虽然 Cloudflare 默认 HTTPS 重定向，但直接访问源站 `http://192.168.31.2:8099` 时无保护）
- **修复建议**：HTTPS 请求时添加 `Strict-Transport-Security: max-age=31536000; includeSubDomains`

### 🟢 Low / 信息类（可接受或需知晓）

| 项目 | 说明 | 修复建议 |
|------|------|----------|
| `nowUnix` | 暴露服务器精确时间 | 几乎无法利用，可忽略 |
| `ffmpegAvailable` | 暴露 FFmpeg 存在性 | 版本未知，输入已受控，可忽略 |
| 本机 CF 头伪造 | 本机进程伪造 `CF-Connecting-IP` | 结果是**权限降级**（Local→Remote），无害 |
| `X-XSS-Protection` 已废弃 | `1; mode=block` 在某些浏览器配置下反而引入 XSS | 移除该头，依赖 CSP |
| GitHub 链接泄露仓库 | `web/index.html` 包含 `github.com/blycr/msp` | 攻击者可读源码找漏洞；移除或动态化 |
| 源站无 TLS | LAN 访问为明文 HTTP | 依赖可信局域网；如需可加自签名 TLS |
| 转码槽饥饿 | 单客户端可占满全部转码并发槽 | 添加 per-IP 转码限制（如最多 1 个） |
| Session 无 IP 绑定 | Cookie 被盗可在别处使用 | 风险低（HttpOnly+Secure）；如需可绑定 IP |
| 日志文件权限 | `logs/msp.log` 权限 `644` | 多用户环境设为 `600` |

---

## 3. 专项审计结论

### 3.1 路径遍历 / 任意文件读取

**基本结论：✅ 安全（存在 TOCTOU 竞态窗口，见 2.3）**

```
/api/stream?id=xxx
  → DecodeID (base64URL)
  → NormalizePath (Clean + Abs)
  → IsAllowedFile
    → EvalSymlinks (解析符号链接真实路径)
    → WithinRoot (检查是否在 shares 目录内)
  → os.Open
```

静态路径验证链是正确的，但**检查与打开之间存在竞态窗口**。

### 3.2 命令注入（FFmpeg）

**结论：✅ 安全**

- `inputPath` 已在上游通过 `IsAllowedFile` 验证
- `opts.Format` 白名单：仅允许 `mp4/mp3/aac/webm/ogg`
- `opts.Bitrate` 正则验证：仅允许数字 + `k`/`m`
- `opts.Offset` 为 `float64`，直接 `fmt.Sprintf("%f")`
- 其余所有 FFmpeg 参数均为硬编码字符串

### 3.3 源站直接暴露分析

**当前状态**：
- 光猫和路由器端口转发已关闭
- Windows 防火墙阻止非局域网入站
- IPv6 规则尚未成功配置（`fe80::/10` 不被 `netsh` 接受）

**风险场景**：
1. **通过 Cloudflare Tunnel 访问**：受 Cloudflare Access OTP + 后端三层过滤双重保护，**安全**
2. **局域网直接访问 `192.168.31.2:8099`**：判定为 LAN，能看到更多字段，但 POST 管理 API 被拦截
3. **本机 `127.0.0.1:8099` 访问**：判定为 Local（除非伪造 `CF-Connecting-IP`，则降级为 Remote）
4. **公网直接访问源站**：当前被 Windows 防火墙阻止（假设规则生效），**但如果未来防火墙规则被意外删除或端口转发被重新打开**，攻击者可直接访问源站

**关键弱点**：如果公网直接访问源站成为现实：
- `getClientIP(r, false)` 只读 `RemoteAddr`，**不受代理头影响**
- 攻击者看到 LAN 级别的过滤视图（比 Remote 更宽松）
- 但仍无法 POST 管理 API

---

## 4. 修复优先级建议

| 优先级 | 修复项 | 文件 | 工作量 |
|--------|--------|------|--------|
| **P0** | PIN 速率限制 + 取消默认 PIN | `internal/handler/auth.go` + 限流器 | 中 |
| **P0** | 全局限流器（保护 refresh/stream/pin） | `internal/handler/middleware.go` | 中 |
| **P1** | TOCTOU 修复：打开后再验证路径 | `internal/handler/stream.go` | 中 |
| **P1** | Inline XSS：非媒体文件强制 attachment | `internal/handler/stream.go` | 低 |
| **P1** | 刷新冷却：限制 `?refresh=1` 频率 | `internal/handler/media.go` | 低 |
| **P1** | CSP 头 | `internal/handler/middleware.go` | 低 |
| **P1** | LAN 过滤 shares.path / port | `internal/handler/config.go` | 低 |
| **P2** | `constantTimeCompare` → `crypto/subtle` | `internal/handler/auth.go` | 低 |
| **P2** | 客户端日志注入防护 | `internal/handler/progress.go` | 低 |
| **P2** | WriteTimeout | `cmd/msp/main.go` | 低 |
| **P2** | HSTS 头 | `internal/handler/middleware.go` | 低 |
| **P2** | IP 黑白名单识别 CF 真实 IP | `internal/handler/middleware.go` | 低 |
| **P3** | Base64 ID → HMAC/DB 映射 | `internal/util/util.go` + 数据库 | 高 |
| **P3** | 移除废弃 `X-XSS-Protection` | `internal/handler/middleware.go` | 低 |
| **P3** | GitHub 链接动态化/移除 | `web/index.html` | 低 |

---

## 5. 访问分级行为速查表

| 场景 | 判定方式 | GET `/api/config` | POST `/api/config` | `/api/ip` |
|------|---------|-------------------|-------------------|-----------|
| 本机浏览器直连 `127.0.0.1` | `RemoteAddr=127.0.0.1` + 无 CF 头 | 完整数据 | ✅ 允许 | 完整 |
| 本机 + 伪造 `CF-Connecting-IP` | `RemoteAddr=127.0.0.1` + CF 头存在 | Remote 过滤 | ❌ 403 | `[]` |
| 局域网 `192.168.31.x` | `RemoteAddr=192.168.31.x` | LAN 过滤 | ❌ 403 | 完整 |
| Cloudflare Tunnel | `RemoteAddr=127.0.0.1` + CF 头存在 | Remote 过滤 | ❌ 403 | `[]` |
| 公网直连源站（假设防火墙失效） | `RemoteAddr=公网IP` | Remote 过滤 | ❌ 403 | `[]` |

---

## 6. 关键代码位置速查

```
访问分级判定       → internal/handler/middleware.go  getAccessLevelFromRequest()
管理 API 拦截      → internal/handler/middleware.go  WithAdminLockdown() / isAdminAPI()
后端字段过滤       → internal/handler/config.go      HandleConfig() GET 分支
PIN 验证          → internal/handler/auth.go        HandlePIN()
Session 管理      → internal/service/session.go     SessionService
文件流验证        → internal/handler/stream.go      resolveMediaTarget()
路径安全检查      → internal/util/util.go           IsAllowedFile() / WithinRoot()
转码参数安全      → internal/media/transcoder.go    TranscodeOptions.Validate()
安全响应头        → internal/handler/middleware.go  applySecurityHeaders()
错误恢复          → internal/handler/middleware.go  WithRecovery()
客户端日志        → internal/handler/progress.go    HandleLog()
播放进度          → internal/handler/progress.go    HandleProgress() / HandlePrefs()
```
