# 安全功能快速开始指南

本指南将帮助您快速配置和使用 MSP 的安全功能。

## 快速配置

### 步骤 1：找到配置文件

配置文件位于可执行文件同目录下的 `config.json`。

如果文件不存在，首次运行程序时会自动创建。

### 步骤 2：编辑配置文件

使用文本编辑器打开 `config.json`，找到 `security` 部分：

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

### 步骤 3：根据需求配置

#### 场景 A：我想只允许本地网络访问

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

**说明：**
- `127.0.0.1` 允许本机访问
- `192.168.1.0/24` 允许 192.168.1.x 网段的所有设备访问
- 根据您的网络情况修改 IP 范围

#### 场景 B：我想添加 PIN 保护

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

**说明：**
- 将 `pinEnabled` 设为 `true`
- 将 `pin` 改为您想要的密码（建议 4-6 位数字）
- 所有访问都需要输入正确的 PIN

#### 场景 C：我想阻止某个 IP 访问

```json
{
  "security": {
    "ipWhitelist": [],
    "ipBlacklist": ["192.168.1.100"],
    "pinEnabled": false,
    "pin": "0000"
  }
}
```

**说明：**
- 将要阻止的 IP 添加到 `ipBlacklist` 数组中
- 可以添加多个 IP，用逗号分隔

#### 场景 D：最安全配置（推荐公网使用）

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

**说明：**
- 只允许本地网络访问
- 阻止特定 IP
- 启用 PIN 认证
- 多重保护

### 步骤 4：重启服务

修改配置后，需要重启 MSP 服务才能生效。

## 常见问题

### Q: 如何知道我的 IP 地址？

**A:** 运行 MSP 后，控制台会显示所有可用的访问地址，包括您的局域网 IP。

例如：
```
访问: http://127.0.0.1:8099/
访问: http://192.168.1.50:8099/
```

这里 `192.168.1.50` 就是您的局域网 IP。

### Q: 我忘记了 PIN 码怎么办？

**A:** 有两种方法：

1. **编辑配置文件**：打开 `config.json`，修改 `pin` 值或将 `pinEnabled` 设为 `false`
2. **删除配置文件**：删除 `config.json`，重启程序会使用默认 PIN `0000`

### Q: 配置了白名单后无法访问怎么办？

**A:** 检查以下几点：

1. 确保您的 IP 在白名单中
2. 如果使用 CIDR 范围，确保范围正确
3. 检查是否有代理或 VPN 改变了您的 IP
4. 临时清空白名单 `[]` 以恢复访问

### Q: CIDR 范围怎么写？

**A:** 常用的 CIDR 范围：

- `/24`：匹配 256 个 IP（如 `192.168.1.0/24` 匹配 `192.168.1.0` 到 `192.168.1.255`）
- `/16`：匹配 65536 个 IP（如 `192.168.0.0/16` 匹配 `192.168.0.0` 到 `192.168.255.255`）
- `/8`：匹配 16777216 个 IP（如 `10.0.0.0/8` 匹配 `10.0.0.0` 到 `10.255.255.255`）

### Q: 如何在前端验证 PIN？

**A:** 当启用 PIN 认证后：

1. 访问任何页面时，如果 PIN 无效，会收到 401 错误
2. 调用 `/api/pin` 端点验证 PIN：

```javascript
fetch('/api/pin', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({ pin: '1234' })
})
.then(res => res.json())
.then(data => {
  if (data.valid) {
    console.log('PIN 验证成功');
    // 验证成功后，cookie 会自动设置，后续请求无需再次验证
  } else {
    console.log('PIN 验证失败');
  }
});
```

3. 也可以在请求头中传递 PIN：

```javascript
fetch('/api/config', {
  headers: {
    'X-PIN': '1234'
  }
})
```

## 配置模板

### 家庭局域网

```json
{
  "security": {
    "ipWhitelist": ["192.168.1.0/24"],
    "ipBlacklist": [],
    "pinEnabled": false,
    "pin": "0000"
  }
}
```

### 小型办公室

```json
{
  "security": {
    "ipWhitelist": ["192.168.1.0/24", "192.168.2.0/24"],
    "ipBlacklist": [],
    "pinEnabled": true,
    "pin": "office2024"
  }
}
```

### 公网访问（最高安全）

```json
{
  "security": {
    "ipWhitelist": ["你的公网IP"],
    "ipBlacklist": [],
    "pinEnabled": true,
    "pin": "strong_password_123"
  }
}
```

### 开发测试（无限制）

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

## 下一步

- 查看完整的[安全配置指南](./SECURITY.md)
- 了解所有[配置选项](./CONFIG_EXAMPLE.md)
- 阅读[更新说明](./SECURITY_UPDATE.md)

## 获取帮助

如果遇到问题，请：

1. 检查配置文件格式是否正确（JSON 格式）
2. 查看服务器日志文件 `msp.log`
3. 在 GitHub 上提交 Issue
