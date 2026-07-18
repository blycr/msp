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
      "playback": {
        "video": {
          "enabled": true,
          "scope": "folder",
          "transcode": true,
          "resume": true,
          "encoding": {
            "hwAccel": "auto",
            "maxJobs": 0
          }
        },
        "audio": { ... },
        "image": { ... }
      }
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
- **响应**: `SharesOpResponse`
  ```json
  {
    "config": { ... }
  }
  ```

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
        "ext": ".mp4",
        "kind": "video",
        "size": 1024000,
        "modTime": 1700000000,
        "shareLabel": "电影",
        "subtitles": [...]
      }
    ],
    "audios": [...],
    "images": [...],
    "others": [...],
    "shares": [...],
    "scanning": false
  }
  ```
- **当 `limit` 参数生效时**，响应会额外包含以下字段：
  - `limited`: `true` — 表示结果被截断
  - `videosTotal` / `audiosTotal` / `imagesTotal` / `othersTotal` — 各分类的真实总数

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
- **缓存**: 原始流响应携带 `Last-Modified`，支持 `If-Modified-Since` 条件请求（未修改返回 304）。转码流为动态内容，恒 `Cache-Control: no-store`。
- **播放策略说明**（v1.2.0 更新）:
  - 默认优先返回原始流（直连），需要转码时前端传递 `transcode=1`。
  - 转码需在配置中开启 `playback.video.transcode`（视频）或 `playback.audio.transcode`（音频）。
  - 前端通过 `GET /api/probe` 获取 `playback.mode` 字段决定是否转码，基于实际编码而非扩展名。
  - 支持硬件加速转码（NVIDIA/Intel/AMD/VAAPI/VideoToolbox），详见 `playback.video.encoding` 配置。
  - FFmpeg 路径支持 7 层搜索（环境变量 → 可执行文件目录 → bin/ → 平台路径 → PATH），详见 `MSP_FFMPEG_PATH` 环境变量。
  - `.wmv` 原始流响应头为 `video/x-ms-wmv`。

### 字幕流
获取外挂字幕文件。支持 SRT、ASS、SSA 格式，自动转换为 WebVTT。

- **端点**: `GET /api/subtitle`
- **参数**:
  - `id`: 字幕文件 ID (注意：这是字幕文件的 ID，不是视频 ID)。
- **响应**: `text/vtt` 内容。
- **补充**: 也支持 `HEAD` 请求（仅返回头信息，不返回内容）。

### 媒体探针
获取媒体文件的详细元数据（编码格式、流信息）和推荐播放策略。

- **端点**: `GET /api/probe`
- **参数**:
  - `id`: 媒体文件 ID。
- **响应**: `ProbeResponse`
  ```json
  {
    "container": "mkv",
    "video": "H.265/HEVC",
    "audio": "AC-3",
    "subtitles": [...],
    "playback": {
      "mode": "transcode"
    }
  }
  ```
- **`playback` 字段**（v1.2.0+）：
  - 仅在对应类型的转码配置开启时返回（`playback.video.transcode` 或 `playback.audio.transcode`）
  - `mode: "direct"` — 浏览器可直接播放（H.264/AAC/MP3/Opus 等）
  - `mode: "transcode"` — 需要服务端转码（HEVC/AV1/VC-1/AC-3/DTS/TrueHD 等）
  - 判断基于实际编码信息（字节嗅探 + ffprobe），非文件扩展名
  - 字段为 `omitempty`，旧客户端忽略即可，向后兼容
- **编码兼容性参考**:
  - 浏览器原生支持的视频编码：H.264/AVC
  - 浏览器原生支持的音频编码：AAC、MP3、Opus、Vorbis、FLAC
  - 需要转码的视频编码：H.265/HEVC、AV1、VC-1/WMV3、未知编码
  - 需要转码的音频编码：AC-3、E-AC-3、DTS、TrueHD、未知编码

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
- **请求体**: `{"pin": "1234"}`（PIN 必须为 4-8 位数字）
- **响应**:
  ```json
  {
    "valid": true,
    "enabled": true
  }
  ```
- **补充说明**:
  - `pinEnabled=false` 时，返回 `{"valid": true, "enabled": false}`。
  - 会话通过 cookie `msp_session` 或请求头 `X-Session-Token` 传递。
  - PIN 认证仅作用于 `/api/*`，其中 `/api/pin`、`/api/ip`、`/api/config` 为豁免端点。

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
- **响应**: 204 No Content
