# MSP 配置文件示例（带注释）

此文件展示了所有可用的配置选项及其说明。
注意：实际的 config.json 文件不支持注释，这只是一个参考文档。

## 基本配置

```json
{
  // 日志级别：debug, info, error, none
  "logLevel": "info",
  
  // 日志文件路径（相对于可执行文件目录）
  "logFile": "msp.log",
  
  // 服务器端口
  "port": 8099,
  
  // 最大扫描项目数（0 表示无限制）
  "maxItems": 0,
  
  // 共享目录列表
  "shares": [],
```

## 功能配置

```json
  "features": {
    // 是否启用播放速度控制
    "speed": true,
    
    // 可选的播放速度选项
    "speedOptions": [0.5, 0.75, 1, 1.25, 1.5, 2],
    
    // 是否启用画质选择
    "quality": false,
    
    // 是否启用字幕功能
    "captions": true,
    
    // 是否启用播放列表
    "playlist": true
  },
```

## UI 配置

```json
  "ui": {
    // 默认标签页：video, audio, image
    "defaultTab": "video",
    
    // 是否显示其他文件类型
    "showOthers": false
  },
```

## 播放配置

```json
  "playback": {
    "audio": {
      // 是否启用音频播放
      "enabled": true,
      
      // 是否默认随机播放
      "shuffle": false,
      
      // 是否记住播放位置
      "remember": true,
      
      // 播放范围：all（全部）或 folder（当前文件夹）
      "scope": "all"
    },
    "video": {
      "enabled": true,
      "scope": "folder",
      
      // 是否启用转码（推荐开启）
      // 开启后，如果浏览器无法直接播放（如 MKV, AVI, FLAC），
      // 前端会自动回退请求后端进行实时转码。
      // 如果关闭，不支持的格式将无法播放。
      "transcode": true
    },
    "image": {
      "enabled": true,
      "scope": "folder"
    }
  },
```

## 黑名单配置

```json
  "blacklist": {
    // 要排除的文件扩展名（不含点号）
    "extensions": [],
    
    // 要排除的文件名
    "filenames": [],
    
    // 要排除的文件夹名
    "folders": [],
    
    // 文件大小规则（例如：">100MB" 或 "<1KB"）
    "sizeRule": ""
  },
```

## 安全配置

```json
  "security": {
    // IP 白名单：只允许这些 IP 访问（为空则不限制）
    // 支持单个 IP 或 CIDR 范围
    // 示例：["192.168.1.100", "192.168.1.0/24", "10.0.0.0/8"]
    "ipWhitelist": [],
    
    // IP 黑名单：禁止这些 IP 访问
    // 支持单个 IP 或 CIDR 范围
    // 示例：["192.168.1.50", "172.16.0.0/16"]
    "ipBlacklist": [],
    
    // 是否启用 PIN 认证
    "pinEnabled": false,
    
    // PIN 码（默认：0000）
    // 可以是任意字符串，建议使用 4-6 位数字
    "pin": "0000"
  }
}
```

## 安全配置使用场景

### 场景 1：仅允许本地网络访问

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

### 场景 2：启用 PIN 认证（适合公网访问）

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

### 场景 3：组合使用（最安全）

```json
{
  "security": {
    "ipWhitelist": ["192.168.1.0/24"],
    "ipBlacklist": ["192.168.1.100"],
    "pinEnabled": true,
    "pin": "1234"
  }
}
```

## 注意事项

1. **IP 过滤优先级**：
   - 如果设置了白名单，只有白名单中的 IP 可以访问
   - 黑名单优先级更高：即使 IP 在白名单中，如果也在黑名单中，仍会被拒绝

2. **CIDR 支持**：
   - `/8`：匹配整个 A 类网络（如 10.0.0.0/8 匹配 10.x.x.x）
   - `/16`：匹配整个 B 类网络（如 192.168.0.0/16 匹配 192.168.x.x）
   - `/24`：匹配整个 C 类网络（如 192.168.1.0/24 匹配 192.168.1.x）

3. **PIN 认证**：
   - PIN 验证成功后会设置 cookie，有效期 7 天
   - 可以通过 `X-PIN` 请求头或 cookie 传递 PIN
   - `/api/pin` 端点用于验证 PIN，不需要 PIN 认证

4. **修改配置**：
   - 修改配置文件后需要重启服务
   - 如果忘记 PIN，可以编辑 config.json 文件修改或禁用

详细文档请参阅：docs/SECURITY.md
