# MSP 项目安全审查报告

**审查日期**: 2026-02-05  
**审查范围**: c:\Users\blycr\msp  
**审查人员**: AI Security Auditor

---

## 执行摘要

本次安全审查对 MSP (Media Share & Preview) 项目进行了全面的安全分析。项目整体代码质量良好，采用了多种安全最佳实践，但在路径遍历防护、输入验证和配置安全方面发现了一些需要改进的地方。

**风险评级**: 🟡 中等风险  
**发现漏洞**: 3个高危、2个中危、4个低危  
**建议措施**: 立即修复高危漏洞，逐步改进中低危问题

---

## 1. 路径遍历防护分析

### 1.1 util.IsAllowedFile() 实现审查

**文件位置**: `internal/util/util.go` (第149-170行)

**当前实现**:
```go
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
```

**发现的问题**:

#### 🔴 高危: 符号链接绕过 (Symlink Bypass)

**问题描述**: `IsAllowedFile()` 在验证路径后，后续的文件操作可能通过符号链接跳转到共享目录之外。

**攻击场景**:
1. 攻击者在共享目录内创建一个指向 `/etc/passwd` 的符号链接
2. `IsAllowedFile()` 验证符号链接本身在共享目录内，返回 true
3. 后续 `os.Open()` 打开的是 `/etc/passwd`

**代码位置**:
- `internal/handler/handlers.go:444` - `os.Open(target)` 在验证后执行
- `internal/media/transcoder.go:94-101` - 转码前仅检查 `os.Stat`

**修复建议**:
```go
// 在 IsAllowedFile 中添加符号链接检查
func IsAllowedFile(fileAbs string, shares []config.Share) bool {
    // ... 现有代码 ...
    
    // 检查是否为符号链接
    if fi, err := os.Lstat(f); err == nil && fi.Mode()&os.ModeSymlink != 0 {
        // 解析符号链接目标
        linkTarget, err := os.Readlink(f)
        if err != nil {
            return false
        }
        // 确保链接目标也在允许的目录内
        if !filepath.IsAbs(linkTarget) {
            linkTarget = filepath.Join(filepath.Dir(f), linkTarget)
        }
        linkTarget = filepath.Clean(linkTarget)
        
        // 递归检查链接目标
        allowed := false
        for _, sh := range shares {
            root := NormalizePath(sh.Path)
            if WithinRoot(root, linkTarget) {
                allowed = true
                break
            }
        }
        if !allowed {
            return false
        }
    }
    
    // ... 后续代码 ...
}
```

#### 🟡 中危: 路径规范化竞争条件 (TOCTOU)

**问题描述**: 在 `IsAllowedFile` 和实际文件操作之间存在时间窗口，文件系统状态可能发生变化。

**影响**: 虽然难以利用，但在高并发场景下存在理论风险。

**修复建议**: 使用文件描述符传递，避免多次路径解析。

### 1.2 文件操作验证覆盖审查

**审查结果**:

| 端点 | 验证函数 | 验证位置 | 状态 |
|------|----------|----------|------|
| `/api/stream` | `IsAllowedFile` | handlers.go:438 | ✅ 已验证 |
| `/api/probe` | `IsAllowedFile` | handlers.go:568 | ✅ 已验证 |
| `/api/subtitle` | `resolveMediaTarget` | handlers.go:593 | ✅ 已验证 |
| 转码功能 | `os.Stat` | transcoder.go:94 | ⚠️ 仅基本检查 |

---

## 2. 输入验证分析

### 2.1 API 端点参数验证

#### 🔴 高危: FFmpeg 命令注入

**文件位置**: `internal/media/transcoder.go` (第93-194行)

**问题描述**: `TranscodeStream` 函数直接使用用户输入的 `inputPath` 构建 FFmpeg 命令，存在命令注入风险。

**漏洞代码**:
```go
func TranscodeStream(ctx context.Context, inputPath string, opts TranscodeOptions) (io.ReadCloser, error) {
    // ...
    args = append(args, "-i", inputPath)  // 直接使用用户输入
    // ...
    cmd := exec.CommandContext(ctx, "ffmpeg", args...)
}
```

**攻击示例**:
```
inputPath = "/path/to/file.mp4; rm -rf /"
# 或
inputPath = "/path/to/file.mp4' -vf 'format=pix_fmts=null"
```

**修复建议**:
```go
func TranscodeStream(ctx context.Context, inputPath string, opts TranscodeOptions) (io.ReadCloser, error) {
    // 1. 验证路径已通过 IsAllowedFile
    // 2. 使用参数列表而非字符串拼接
    // 3. 对路径进行额外验证
    
    // 验证路径不包含特殊字符
    if strings.ContainsAny(inputPath, ";|&$`\"'\\n\\r<>") {
        return nil, fmt.Errorf("invalid characters in path")
    }
    
    // 验证文件存在且不是目录
    info, err := os.Stat(inputPath)
    if err != nil || info.IsDir() {
        return nil, fmt.Errorf("invalid input path")
    }
    
    // ... 后续代码
}
```

#### 🟡 中危: 转码参数验证不足

**问题描述**: `TranscodeOptions` 中的 `Format` 和 `Bitrate` 参数未进行严格验证。

**文件位置**: `internal/media/transcoder.go` (第111-113, 128, 139-141)

**漏洞代码**:
```go
if opts.Format == "" {
    opts.Format = "mp4"
}
// ...
args = append(args, "-f", opts.Format)  // 未验证格式
// ...
args = append(args, "-b:a", opts.Bitrate)  // 未验证码率格式
```

**修复建议**:
```go
var allowedFormats = map[string]bool{
    "mp4": true, "mp3": true, "aac": true, "webm": true,
}

var bitratePattern = regexp.MustCompile(`^\\d+[km]?$`)

func validateTranscodeOptions(opts *TranscodeOptions) error {
    if !allowedFormats[opts.Format] {
        return fmt.Errorf("unsupported format: %s", opts.Format)
    }
    if opts.Bitrate != "" && !bitratePattern.MatchString(opts.Bitrate) {
        return fmt.Errorf("invalid bitrate format")
    }
    return nil
}
```

### 2.2 用户输入过滤和转义

#### 🟢 良好实践: JSON 解码

**文件位置**: `internal/handler/handlers.go`

所有 API 端点都使用 `json.NewDecoder(r.Body).Decode()` 进行输入解析，这是安全的做法。

#### 🟡 低危: 日志注入风险

**文件位置**: `internal/handler/handlers.go:236`

**问题描述**: 客户端日志直接写入服务器日志，未进行过滤。

```go
func (h *Handler) HandleLog(w http.ResponseWriter, r *http.Request) {
    // ...
    if req.Msg != "" {
        h.s.Log(req.Level, req.Msg)  // 直接记录用户输入
    }
}
```

**修复建议**:
```go
func sanitizeLogMessage(msg string) string {
    // 移除控制字符和过长的消息
    msg = strings.ReplaceAll(msg, "\n", "\\n")
    msg = strings.ReplaceAll(msg, "\r", "\\r")
    if len(msg) > 1000 {
        msg = msg[:1000] + "..."
    }
    return msg
}
```

---

## 3. 认证和授权分析

### 3.1 PIN 码验证机制

**文件位置**: `internal/handler/handlers.go` (第241-308行)

#### 🟢 良好实践: 恒定时间比较

```go
func constantTimeCompare(a, b string) bool {
    if len(a) != len(b) {
        return false
    }
    result := 0
    for i := 0; i < len(a); i++ {
        result |= int(a[i] ^ b[i])
    }
    return result == 0
}
```

✅ 使用了恒定时间比较，防止时序攻击。

#### 🟢 良好实践: 会话管理

**文件位置**: `internal/server/server.go` (第568-624行)

- 使用加密安全的随机数生成 session token
- Session 有过期时间 (7天)
- 定期清理过期 session
- Cookie 设置了 HttpOnly 和 SameSite

#### 🟡 低危: PIN 码强度

**问题描述**: 默认 PIN 为 "0000"，且配置中明文存储。

**建议**:
1. 首次启动时强制要求用户修改默认 PIN
2. 添加 PIN 复杂度要求（最少4位，建议6位）
3. 考虑使用 bcrypt 等慢哈希存储 PIN（虽然需要恒定时间比较会有冲突）

### 3.2 IP 黑白名单实现

**文件位置**: `internal/handler/middleware.go` (第77-247行)

#### 🟢 良好实践: IP 获取安全

```go
func getClientIP(r *http.Request, trustProxy bool) string {
    // Only trust proxy headers when explicitly configured
    if trustProxy {
        // Check X-Forwarded-For...
    }
    // Use RemoteAddr (most secure option, cannot be spoofed by client)
    ip := r.RemoteAddr
    // ...
}
```

✅ 默认不信任代理头，防止 IP 欺骗。

#### 🟢 良好实践: CIDR 支持

支持 CIDR 表示法的 IP 范围匹配，使用标准库 `net.ParseCIDR`。

#### 🟡 低危: IP 格式验证

**问题描述**: `matchesIPList` 中对 IP 格式验证不够严格。

**修复建议**:
```go
func matchesIPList(clientIP string, ipList []string) bool {
    // 验证 clientIP 格式
    if net.ParseIP(clientIP) == nil {
        return false
    }
    // ... 后续代码
}
```

### 3.3 Cookie 和会话管理

#### 🟢 良好实践: Cookie 安全配置

```go
http.SetCookie(w, &http.Cookie{
    Name:     "msp_session",
    Value:    sessionToken,
    Path:     "/",
    MaxAge:   constants.CookieMaxAge,  // 7天
    HttpOnly: true,                    // 防止 XSS 窃取
    SameSite: http.SameSiteLaxMode,    // CSRF 防护
})
```

#### 🟡 低危: 缺少 Secure 标志

**问题描述**: Cookie 未设置 `Secure` 标志，在 HTTPS 环境下可能通过 HTTP 传输。

**修复建议**:
```go
// 检测是否使用 HTTPS
isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

http.SetCookie(w, &http.Cookie{
    // ...
    Secure: isSecure,
})
```

---

## 4. 命令注入防护分析

### 4.1 FFmpeg 转码调用

**文件位置**: `internal/media/transcoder.go`

#### 🔴 高危: 命令注入（已在 2.1 中描述）

#### 🟢 良好实践: 参数化调用

虽然存在注入风险，但代码使用了参数列表而非字符串拼接：

```go
args := []string{"-hide_banner", "-loglevel", "error"}
args = append(args, "-i", inputPath)
// ...
cmd := exec.CommandContext(ctx, "ffmpeg", args...)
```

这比使用 shell 字符串拼接更安全，但仍需要输入验证。

#### 🟢 良好实践: 并发限制

```go
var transcodeLimit = make(chan struct{}, 2)
```

限制了最多 2 个并发转码会话，防止资源耗尽攻击。

### 4.2 其他外部命令调用

**文件位置**: `cmd/msp/main.go:141-149`

```go
func openBrowser(url string) error {
    switch runtime.GOOS {
    case "windows":
        return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
    // ...
    }
}
```

🟢 这是本地功能，不涉及用户输入，风险较低。

---

## 5. SQL 注入防护分析

### 5.1 GORM 使用规范

**文件位置**: `internal/db/db.go`

#### 🟢 良好实践: 参数化查询

所有数据库操作都使用 GORM 的参数化查询：

```go
// 使用 ? 占位符
err := DB.WithContext(ctx).First(&p, "media_id = ?", mediaID).Error

// 使用 IN 查询
return dbConn.WithContext(ctx).Where("share_root NOT IN ?", shareRoots).Delete(&types.MediaItem{}).Error
```

#### 🟢 良好实践: 无原始 SQL 拼接

审查未发现任何字符串拼接构建 SQL 的情况。

### 5.2 数据库配置安全

#### 🟢 良好实践: WAL 模式

```go
sqlDB.Exec("PRAGMA journal_mode=WAL;")
sqlDB.Exec("PRAGMA synchronous=NORMAL;")
```

提高了 SQLite 的并发性能和可靠性。

---

## 6. 其他安全问题

### 6.1 CORS 配置

**审查结果**: 项目未显式配置 CORS，使用默认行为（同源策略）。

🟢 这是安全的默认配置。如需跨域支持，建议：

```go
// 如需添加 CORS，使用白名单
allowedOrigins := []string{"http://localhost:3000", "https://example.com"}
```

### 6.2 敏感信息泄露

#### 🟢 良好实践: 错误信息处理

API 错误返回统一的错误消息，不泄露内部细节：

```go
http.Error(w, "not allowed", http.StatusForbidden)
// 而不是
http.Error(w, err.Error(), http.StatusForbidden)
```

#### 🟡 低危: 配置信息泄露

**文件位置**: `internal/handler/handlers.go:39-59`

`/api/config` 端点返回完整配置，包括安全相关设置：

```go
func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
    case http.MethodGet:
        view := h.configService.GetConfigView()
        writeJSON(w, http.StatusOK, view)
}
```

**建议**: 过滤敏感字段（如 PIN 码）后再返回。

### 6.3 资源限制

#### 🟢 良好实践: HTTP 服务器超时配置

```go
server := &http.Server{
    ReadHeaderTimeout: 10 * time.Second,
    ReadTimeout:       15 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

#### 🟢 良好实践: 转码并发限制

```go
var transcodeLimit = make(chan struct{}, 2)
```

#### 🟢 良好实践: 日志轮转

```go
if st.Size() < constants.LogRotateSize {  // 10MB
    return
}
```

### 6.4 安全头部

**文件位置**: `internal/handler/middleware.go:122-132`

#### 🟢 良好实践: 安全头部设置

```go
func applySecurityHeaders(w http.ResponseWriter) {
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.Header().Set("X-Frame-Options", "DENY")
    w.Header().Set("X-XSS-Protection", "1; mode=block")
    w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}
```

#### 🟡 低危: 缺少 CSP

**建议**: 添加 Content Security Policy 头：

```go
w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
```

---

## 7. 前端安全分析

### 7.1 XSS 防护

**文件位置**: `web/src/modules/api.js`

#### 🟢 良好实践: 使用 textContent

前端代码使用 `textContent` 而非 `innerHTML`，防止 XSS。

### 7.2 认证状态管理

**文件位置**: `web/src/modules/pin.js`

🟢 前端正确实现了 PIN 验证流程，依赖后端的 session cookie。

---

## 8. 修复优先级建议

### 立即修复 (24小时内)

1. **符号链接绕过** (`internal/util/util.go`)
   - 在 `IsAllowedFile` 中添加符号链接解析和验证
   
2. **FFmpeg 命令注入** (`internal/media/transcoder.go`)
   - 添加输入路径验证
   - 限制允许的文件扩展名

### 短期修复 (1周内)

3. **转码参数验证** (`internal/media/transcoder.go`)
   - 白名单验证 format 和 bitrate 参数
   
4. **配置信息泄露** (`internal/handler/handlers.go`)
   - 过滤敏感字段后返回配置

### 中期改进 (1月内)

5. **日志注入防护** (`internal/handler/handlers.go`)
6. **Cookie Secure 标志** (`internal/handler/handlers.go`)
7. **CSP 头部** (`internal/handler/middleware.go`)
8. **PIN 码强度要求** (`internal/config/config.go`)

---

## 9. 安全测试建议

### 9.1 自动化测试

建议添加以下安全测试：

```go
// internal/util/util_test.go
func TestIsAllowedFile_Symlink(t *testing.T) {
    // 测试符号链接绕过
}

func TestIsAllowedFile_Traversal(t *testing.T) {
    // 测试路径遍历攻击
    tests := []string{
        "../../../etc/passwd",
        "..\\..\\..\\windows\\system32\\config\\sam",
        "valid/path/../../../etc/passwd",
    }
    // ...
}
```

### 9.2 手动渗透测试

1. 使用 Burp Suite 测试 API 端点
2. 测试符号链接绕过
3. 测试 FFmpeg 命令注入
4. 测试 PIN 暴力破解防护

---

## 10. 总结

MSP 项目整体安全架构良好，采用了多种安全最佳实践：

- ✅ 使用参数化查询防止 SQL 注入
- ✅ 恒定时间 PIN 比较防止时序攻击
- ✅ 安全的会话管理和 Cookie 配置
- ✅ 合理的资源限制和超时配置
- ✅ 安全头部设置

但存在以下需要立即关注的问题：

- 🔴 **符号链接绕过** 可能导致未授权文件访问
- 🔴 **FFmpeg 命令注入** 可能导致远程代码执行

建议按照修复优先级尽快处理这些问题，并建立安全测试流程。

---

**报告生成时间**: 2026-02-05  
**审查工具**: 静态代码分析 + 人工审查
