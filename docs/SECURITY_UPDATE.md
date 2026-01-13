# 安全功能更新说明

## 概述

本次更新为 MSP 项目添加了基于 IP 的黑/白名单机制和 PIN 认证功能，增强了应用的安全性。

## 新增功能

### 1. IP 黑/白名单

- **IP 白名单**：只允许指定的 IP 地址或 IP 范围访问服务器
- **IP 黑名单**：阻止指定的 IP 地址或 IP 范围访问服务器
- **CIDR 支持**：支持 CIDR 表示法（/8, /16, /24）来定义 IP 范围

### 2. PIN 认证

- **PIN 码验证**：要求用户输入正确的 PIN 码才能访问服务
- **默认 PIN**：初始 PIN 码为 `0000`
- **Cookie 支持**：验证成功后自动设置 cookie，7 天内无需重复输入
- **灵活配置**：可在配置文件中自定义 PIN 码

## 修改的文件

### 核心代码

1. **`internal/config/config.go`**
   - 添加 `SecurityConfig` 结构体
   - 在 `Config` 结构体中添加 `Security` 字段
   - 更新 `Default()` 和 `ApplyDefaults()` 函数

2. **`internal/handler/middleware.go`**
   - 添加 `WithSecurity()` 中间件函数
   - 实现 IP 过滤逻辑（`isIPAllowed()`, `matchesIPList()`, `matchesCIDR()`）
   - 实现 PIN 认证逻辑
   - 添加 `getClientIP()` 函数提取真实客户端 IP

3. **`internal/handler/handlers.go`**
   - 添加 `HandlePIN()` 函数处理 PIN 验证请求

4. **`cmd/msp/main.go`**
   - 在中间件链中添加 `WithSecurity()` 中间件
   - 注册 `/api/pin` API 端点

### 配置文件

5. **`config.example.json`**
   - 添加 `security` 配置部分
   - 包含 `ipWhitelist`、`ipBlacklist`、`pinEnabled`、`pin` 字段

### 测试文件

6. **`internal/handler/middleware_test.go`** (新建)
   - IP 过滤功能测试
   - CIDR 匹配测试
   - 客户端 IP 提取测试
   - PIN 认证测试
   - 安全中间件集成测试

### 文档

7. **`docs/SECURITY.md`** (新建)
   - 详细的安全配置指南
   - 配置示例和使用场景
   - API 端点说明

8. **`docs/CONFIG_EXAMPLE.md`** (新建)
   - 带注释的配置文件示例
   - 各配置项的详细说明

## 配置示例

### 基本配置

```json
{
  "security": {
    "ipWhitelist": [],
    "ipBlacklist": [],
    "pinEnabled": false,
    "pin": "0000"
  }
}
```

### 仅允许本地网络访问

```json
{
  "security": {
    "ipWhitelist": ["127.0.0.1", "192.168.1.0/24"],
    "ipBlacklist": [],
    "pinEnabled": false,
    "pin": "0000"
  }
}
```

### 启用 PIN 认证

```json
{
  "security": {
    "ipWhitelist": [],
    "ipBlacklist": [],
    "pinEnabled": true,
    "pin": "1234"
  }
}
```

### 组合使用（推荐用于公网）

```json
{
  "security": {
    "ipWhitelist": ["192.168.1.0/24"],
    "ipBlacklist": ["192.168.1.100"],
    "pinEnabled": true,
    "pin": "8888"
  }
}
```

## API 端点

### POST /api/pin

验证 PIN 码

**请求体：**
```json
{
  "pin": "1234"
}
```

**响应（成功）：**
```json
{
  "valid": true,
  "enabled": true
}
```

**响应（失败）：**
```json
{
  "valid": false,
  "enabled": true
}
```

## 工作原理

### IP 过滤

1. 从请求中提取客户端 IP（优先级：X-Forwarded-For > X-Real-IP > RemoteAddr）
2. 如果配置了白名单，检查 IP 是否在白名单中
3. 检查 IP 是否在黑名单中
4. 黑名单优先级高于白名单

### PIN 认证

1. 检查是否启用 PIN 认证
2. 某些端点（如 `/api/pin`、`/favicon.ico`）不需要 PIN
3. 从请求头 `X-PIN` 或 cookie `msp_pin` 中获取 PIN
4. 验证 PIN 是否正确
5. 验证成功后设置 cookie，有效期 7 天

### 中间件链

```
Request → WithLog → WithSecurity → WithGzip → Handler
```

安全中间件在日志中间件之后、Gzip 中间件之前执行。

## 测试

所有新功能都包含了完整的单元测试：

```bash
# 运行所有测试
go test ./...

# 运行安全相关测试
go test ./internal/handler -v

# 运行配置测试
go test ./internal/config -v
```

## 使用建议

1. **开发环境**：可以不启用任何安全功能
2. **局域网环境**：建议使用 IP 白名单限制访问范围
3. **公网环境**：强烈建议同时启用 IP 白名单和 PIN 认证
4. **定期更换 PIN**：建议定期更换 PIN 码以提高安全性

## 注意事项

1. 修改配置后需要重启服务才能生效
2. 如果配置了 IP 白名单，请确保包含您自己的 IP
3. 如果忘记 PIN，可以编辑 `config.json` 文件修改或禁用
4. IP 过滤会检查代理服务器传递的真实 IP 地址

## 向后兼容性

- 所有新配置项都有默认值
- 默认情况下不启用任何安全限制
- 现有配置文件会自动应用默认的安全配置
- 不会影响现有功能的使用

## 未来改进

可能的改进方向：

1. 支持更多 CIDR 范围（/32, /0 等）
2. 支持 IPv6 地址过滤
3. 支持多个 PIN 码（不同用户）
4. 添加访问日志和审计功能
5. 支持基于时间的访问控制
6. 集成更强的认证机制（如 OAuth）

## 相关文档

- [安全配置指南](./SECURITY.md)
- [配置文件示例](./CONFIG_EXAMPLE.md)
- [项目 README](../README.md)
