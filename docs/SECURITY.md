# 安全配置指南

本文档介绍 MSP 在家庭局域网模式下的安全配置，包括 IP 黑/白名单和 PIN 认证。

## 配置文件位置

安全配置位于 `config.json` 文件的 `security` 部分。

## 配置选项

### IP 白名单 (ipWhitelist)

IP 白名单用于限制只有特定的 IP 地址可以访问服务器。

**配置示例：**

```json
{
  "security": {
    "ipWhitelist": [
      "192.168.1.100",
      "192.168.1.0/24",
      "10.0.0.0/8"
    ]
  }
}
```

**说明：**
- 如果白名单为空 `[]`，则不进行白名单过滤
- 如果白名单不为空，则只有列表中的 IP 可以访问
- 支持单个 IP 地址（如 `192.168.1.100`）
- 支持 CIDR 范围（如 `192.168.1.0/24`、`10.0.0.0/8`）

**支持的 CIDR 范围：**
- 目前支持 IPv4 CIDR，前缀长度 `0-32`
- 常用示例：`/8`、`/16`、`/24`

### IP 黑名单 (ipBlacklist)

IP 黑名单用于阻止特定的 IP 地址访问服务器。

**配置示例：**

```json
{
  "security": {
    "ipBlacklist": [
      "192.168.1.50",
      "172.16.0.0/16"
    ]
  }
}
```

**说明：**
- 黑名单中的 IP 将被拒绝访问
- 支持单个 IP 地址和 CIDR 范围
- 黑名单优先级：如果 IP 同时在白名单和黑名单中，将被拒绝访问

### PIN 认证 (pinEnabled 和 pin)

PIN 认证要求用户输入正确的 PIN 码才能访问服务。

**配置示例：**

```json
{
  "security": {
    "pinEnabled": true,
    "pin": "1234"
  }
}
```

**说明：**
- `pinEnabled`: 是否启用 PIN 认证（`true` 或 `false`）
- `pin`: PIN 码，默认为 `"0000"`
- PIN 码必须为 **4-8 位数字**（仅 `0-9`）
- 建议使用 6-8 位数字并定期更换

## 完整配置示例

### 示例 1：仅允许本地网络访问

```json
{
  "security": {
    "ipWhitelist": [
      "127.0.0.1",
      "192.168.1.0/24"
    ],
    "ipBlacklist": [],
    "pinEnabled": false,
    "pin": "0000"
  }
}
```

### 示例 2：启用 PIN 认证

```json
{
  "security": {
    "ipWhitelist": [],
    "ipBlacklist": [],
    "pinEnabled": true,
    "pin": "8888"
  }
}
```

### 示例 3：组合使用 IP 白名单和 PIN 认证

```json
{
  "security": {
    "ipWhitelist": [
      "192.168.1.0/24"
    ],
    "ipBlacklist": [
      "192.168.1.100"
    ],
    "pinEnabled": true,
    "pin": "1234"
  }
}
```

这个配置将：
1. 只允许 `192.168.1.x` 网段的 IP 访问
2. 但拒绝 `192.168.1.100` 访问
3. 所有通过 IP 过滤的用户还需要输入 PIN 码 `1234`

## API 端点

### PIN 验证端点

**端点：** `POST /api/pin`

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

**说明：**
- 验证成功后，服务器会设置一个 cookie (`msp_session`)，有效期为 7 天
- 后续请求可以通过 cookie `msp_session` 或请求头 `X-Session-Token` 传递会话令牌
- PIN 认证仅作用于 `/api/*`；`/api/pin`、`/api/ip`、`/api/config` 不需要 PIN

## 使用建议

1. **开发环境**：可以不启用任何安全功能
2. **局域网环境**：建议使用 IP 白名单限制访问范围
3. **公网环境**：当前版本默认面向家庭局域网，不建议直接暴露公网
4. **定期更换 PIN**：建议定期更换 PIN 码以提高安全性

## 注意事项

1. IP 过滤仅使用 `RemoteAddr`（家庭模式不信任代理头）

2. 如果配置了 IP 白名单，请确保包含您自己的 IP，否则您将无法访问服务器

3. 如果忘记 PIN 码，可以：
   - 编辑 `config.json` 文件修改或禁用 PIN
   - 删除 `config.json` 文件，系统将使用默认配置（PIN: `0000`）

4. `trustProxy` 字段仅为配置兼容保留，当前家庭模式不会启用代理头取源 IP
5. 配置支持热重载，保存后通常 2 秒内生效
