# 安全配置指南

本文档介绍 MSP 在家庭局域网模式下的安全配置，包括 IP 黑/白名单和 PIN 认证。

## 快速开始

**需要快速配置？** 请查看 [归档的安全快速开始指南](./archive/SECURITY_QUICKSTART.md) 了解常见场景的配置模板。

---
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

**配置示例（设置 PIN 时写入明文 `pin`，落盘后只会留下 `pinHash`）：**

```json
{
  "security": {
    "pinEnabled": true,
    "pin": "123456"
  }
}
```

**说明：**
- `pinEnabled`: 是否启用 PIN 认证（`true` 或 `false`）
- `pin`: 仅用于**设置/更换** PIN 的瞬时字段；保存时会 bcrypt 哈希到 `pinHash` 并清空 `pin`
- `pinHash`: 持久化的 bcrypt 哈希。默认未启用 PIN，哈希为空
- PIN 码必须为 **4-8 位数字**（仅 `0-9`）
- 建议使用 6-8 位数字并定期更换
- 默认配置**没有** PIN，也没有 `"0000"` 后门

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
    "pinHash": ""
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
   - 编辑 `config.json`：设 `"pinEnabled": false`，或写入新的 `"pin"` 后保存（进程会哈希并清空明文）
   - 删除 `config.json` 后重启：回到默认（PIN **关闭**，不是 `0000`）

4. `trustProxy` 当前不改变取源 IP 的方式。例外：当 `RemoteAddr` 是回环地址且请求带 `CF-Connecting-IP` 时，IP 黑白名单会使用该头（给本机 Cloudflare Tunnel 用）。任意本机进程也可以伪造这个头。
5. `POST /api/config` 与 `POST /api/shares` 仅本机可写。
6. 配置支持热重载，保存后通常 2 秒内生效。非法配置会被拒绝并保留旧配置。
