# MSP 项目安全审查报告

> **⚠️ 历史文档说明**  
> 本文档创建于 2026-02-05，记录了特定时期的安全审查结果。  
> 报告中提到的安全问题已在后续版本中修复。  
> 请以最新代码和实际测试为准，本文档仅供参考。

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

**修复状态**: ✅ 已在后续版本中修复

#### 🟡 中危: 路径规范化竞争条件 (TOCTOU)

**问题描述**: 在 `IsAllowedFile` 和实际文件操作之间存在时间窗口，文件系统状态可能发生变化。

**修复状态**: ✅ 已改进

---

## 2. 输入验证分析

### 2.1 API 端点参数验证

#### 🔴 高危: FFmpeg 命令注入

**文件位置**: `internal/media/transcoder.go` (第93-194行)

**问题描述**: `TranscodeStream` 函数直接使用用户输入的 `inputPath` 构建 FFmpeg 命令，存在命令注入风险。

**修复状态**: ✅ 已添加输入验证和格式白名单

#### 🟡 中危: 转码参数验证不足

**问题描述**: `TranscodeOptions` 中的 `Format` 和 `Bitrate` 参数未进行严格验证。

**修复状态**: ✅ 已添加参数验证

---

## 3. 认证和授权分析

### 3.1 PIN 码验证机制

**良好实践**:
- ✅ 使用了恒定时间比较，防止时序攻击
- ✅ 使用加密安全的随机数生成 session token
- ✅ Session 有过期时间 (7天)
- ✅ Cookie 设置了 HttpOnly 和 SameSite

---

## 4. 命令注入防护分析

### 4.1 FFmpeg 转码调用

**良好实践**:
- ✅ 使用参数列表而非字符串拼接
- ✅ 限制并发转码会话数为 2

---

## 5. SQL 注入防护分析

### 5.1 GORM 使用规范

**良好实践**:
- ✅ 所有数据库操作都使用 GORM 的参数化查询
- ✅ 未发现任何字符串拼接构建 SQL 的情况

---

## 6. 其他安全问题

### 6.1 安全头部

**良好实践**:
```go
func applySecurityHeaders(w http.ResponseWriter) {
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.Header().Set("X-Frame-Options", "DENY")
    w.Header().Set("X-XSS-Protection", "1; mode=block")
    w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}
```

---

## 7. 前端安全分析

### 7.1 XSS 防护

**良好实践**: 前端代码使用 `textContent` 而非 `innerHTML`，防止 XSS。

---

## 8. 修复总结

| 问题 | 严重程度 | 修复状态 |
|------|----------|----------|
| 符号链接绕过 | 🔴 高危 | ✅ 已修复 |
| FFmpeg 命令注入 | 🔴 高危 | ✅ 已修复 |
| 转码参数验证不足 | 🟡 中危 | ✅ 已修复 |
| 配置信息泄露 | 🟡 中危 | ✅ 已修复 |
| 日志注入风险 | 🟢 低危 | ✅ 已改进 |
| Cookie Secure 标志 | 🟢 低危 | ✅ 已添加 |
| CSP 头部 | 🟢 低危 | ✅ 已添加 |
| PIN 码强度要求 | 🟢 低危 | ✅ 已改进 |

---

## 9. 总结

MSP 项目整体安全架构良好，采用了多种安全最佳实践。审查中发现的问题已在后续版本中全部修复。

**当前安全状态**: ✅ 安全

---

**报告生成时间**: 2026-02-05  
**归档时间**: 2026-05-03  
**审查工具**: 静态代码分析 + 人工审查
