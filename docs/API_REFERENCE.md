# MSP API 参考文档

本文档详细描述了 MSP (Media Share & Preview) 服务器提供的 HTTP API 接口。

**基本信息**
- **Base URL**: `/api`
- **数据格式**: JSON (流媒体接口除外)
- **字符编码**: UTF-8

---

## 1. 配置 (Configuration)

### 获取当前配置
获取服务器当前的运行时配置。

- **端点**: `GET /api/config`
- **响应**: `ConfigResponse`
  ```json
  {
    "config": {
      "port": 8099,
      "shares": [...],
      "playback": { ... }
    },
    "lanIPs": ["192.168.1.5"],
    "urls": ["http://192.168.1.5:8099/"],
    "nowUnix": 1705555555
  }
  ```

### 更新配置
全量更新服务器配置。

- **端点**: `POST /api/config`
- **请求体**: `Config` 对象 (JSON)
- **响应**: 成功返回更新后的配置，失败返回错误。

---

## 2. 共享目录 (Shares)

### 管理共享目录
添加或移除媒体共享目录。

- **端点**: `POST /api/shares`
- **请求体**: `SharesOpRequest`
  ```json
  {
    "op": "add",       // "add" 或 "remove"
    "path": "D:/Movies",
    "label": "电影"     // 可选，仅 add 时有效
  }
  ```
- **响应**: `SharesOpResponse` (包含更新后的配置)

---

## 3. 媒体索引 (Media)

### 获取媒体列表
获取所有已索引的媒体文件（视频、音频、图片）。支持缓存和增量更新。

- **端点**: `GET /api/media`
- **参数**:
  - `refresh` (可选): `1` 表示强制重新扫描磁盘（后台进行）。
  - `limit` (可选): 限制返回的项目数量（用于低内存模式）。
- **响应**: `MediaResponse`
  ```json
  {
    "videos": [
      {
        "id": "base64_path...",
        "name": "Movie.mp4",
        "size": 1024000,
        "modTime": 1700000000,
        "subtitles": [...]
      }
    ],
    "audios": [...],
    "images": [...],
    "scanning": false // 如果为 true，表示后台正在扫描，数据可能不完整
  }
  ```

---

## 4. 播放与流媒体 (Streaming)

### 媒体流
获取媒体文件的二进制流。支持 HTTP Range 请求（断点续传/拖动）。

- **端点**: `GET /api/stream`
- **参数**:
  - `id`: 媒体文件 ID (必须)。
  - `transcode`: `1` 请求服务端转码 (默认返回原文件)。
  - `start`: 转码流的起始时间（秒，仅转码模式有效）。
  - `format`: 强制转码格式 (如 `mp4`, `mp3`)。
  - `bitrate`: 限制转码码率 (如 `2M`)。
- **响应**: 二进制媒体流 (video/mp4, audio/mpeg 等)。

### 字幕流
获取外挂字幕文件。会自动将 SRT 转换为 WebVTT 格式。

- **端点**: `GET /api/subtitle`
- **参数**:
  - `id`: 字幕文件 ID (注意：这是字幕文件的 ID，不是视频 ID)。
- **响应**: `text/vtt` 内容。

### 媒体探针
获取媒体文件的详细元数据（编码格式、流信息），用于前端判断是否需要转码。

- **端点**: `GET /api/probe`
- **参数**:
  - `id`: 媒体文件 ID。
- **响应**: `ProbeResponse`
  ```json
  {
    "container": "mkv",
    "video": "H.265/HEVC",
    "audio": "AC-3",
    "subtitles": [...]
  }
  ```

---

## 5. 用户数据 (User Data)

### 获取播放进度
获取单个文件的上次播放进度。

- **端点**: `GET /api/progress`
- **参数**: `id`
- **响应**: `{"time": 120.5}` (秒)

### 保存播放进度
更新文件的播放进度。

- **端点**: `POST /api/progress`
- **请求体**:
  ```json
  {
    "id": "base64_path...",
    "time": 120.5
  }
  ```
- **响应**: 204 No Content

### 获取偏好设置
获取前端存储的所有用户偏好（如音量、主题等）。

- **端点**: `GET /api/prefs`
- **响应**: `{"prefs": {"volume": "0.8", "theme": "dark"}}`

### 更新偏好设置
批量更新用户偏好。

- **端点**: `POST /api/prefs`
- **请求体**:
  ```json
  {
    "prefs": {
      "volume": "1.0"
    }
  }
  ```

---

## 6. 系统与安全 (System & Security)

### 获取局域网 IP
- **端点**: `GET /api/ip`
- **响应**: `{"lanIPs": ["192.168.1.x", ...]}`

### PIN 认证
验证访问 PIN 码。验证成功后会设置 HttpOnly Cookie。

- **端点**: `POST /api/pin`
- **请求体**: `{"pin": "1234"}`
- **响应**:
  ```json
  {
    "valid": true,
    "enabled": true
  }
  ```

### 前端日志上报
允许前端将错误或调试信息发送到后端日志文件。

- **端点**: `POST /api/log`
- **请求体**:
  ```json
  {
    "level": "error",
    "msg": "Player failed to decode..."
  }
  ```
