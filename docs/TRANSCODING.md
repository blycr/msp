# 转码技术文档

本文档详细描述 MSP 的转码架构，包括 FFmpeg 路径发现、播放策略决策、硬件加速和智能编码策略。

---

## 1. 架构概述

MSP 采用 **"后端决策 + 前端执行"** 的播放架构（v1.2.0+）：

```
用户点击媒体
    │
    ▼
前端 getPlaybackUrl(item)
    │
    ├──► GET /api/probe?id=xxx（字节嗅探 ~10-50ms）
    │       └── 后端返回 playback.mode: "direct" | "transcode"
    │
    ├──► "direct" → video.src = streamUrl(id)
    │
    └──► "transcode" → video.src = streamUrl(id) + "&transcode=1"
                            │
                            ▼
                        后端 processor.TranscodeStream()
                            ├── H.264 视频 → stream copy（零开销）
                            ├── 非 H.264 → HW/SW 重编码
                            ├── AAC/MP3 音频 → stream copy
                            └── 其他音频 → 重编码为 AAC
```

**核心优势**：
- 决策基于**实际编码**（字节嗅探），非文件扩展名
- 同一容器（MKV）中的 H.264 和 H.265 会得到不同策略
- 音频编码检查解决"有画无声"——Chrome 不会对 AC-3 抛 error，但后端知道它不兼容
- 首次播放延迟从 5-10 秒（错误回退）降至约 50ms（probe）

---

## 2. FFmpeg 路径发现

### 2.1 搜索优先级

FFmpeg/ffprobe 查找采用 7 层优先级搜索（`internal/media/probe.go`）：

| 优先级 | 位置 | 说明 |
|--------|------|------|
| 1 | `MSP_FFMPEG_PATH` 环境变量 | 用户显式指定，最高优先级 |
| 2 | 可执行文件同目录 | 便携式部署：ffmpeg.exe 和 msp.exe 放一起 |
| 3 | `bin/` 子目录 | 可执行文件的 bin/ 子目录 |
| 4 | 当前工作目录 | `./ffmpeg` |
| 5 | `CWD/bin/` | `./bin/ffmpeg` |
| 6 | 平台特定路径 | Windows: `C:\FFmpeg\bin`；Linux: `/usr/local/bin`；macOS: `/opt/homebrew/bin` |
| 7 | 系统 PATH | `exec.LookPath`，最后回退 |

### 2.2 缓存机制

路径发现通过 `sync.Once` 缓存，全程零重复开销：

```go
type MediaProcessor struct {
    probePaths struct {
        ffmpeg  string
        ffprobe string
        once    sync.Once
    }
    // ... 其他字段
}

func (mp *MediaProcessor) resolveFFmpegPaths() {
    mp.probePaths.once.Do(func() {
        mp.probePaths.ffmpeg = findExecutable("ffmpeg")
        if mp.probePaths.ffmpeg != "" {
            dir := filepath.Dir(mp.probePaths.ffmpeg)
            candidate := filepath.Join(dir, exeName("ffprobe"))
            if _, err := os.Stat(candidate); err == nil {
                mp.probePaths.ffprobe = candidate
            } else {
                mp.probePaths.ffprobe = findExecutable("ffprobe")
            }
        }
    })
}
```

### 2.3 启动日志

启动时会输出 FFmpeg 发现状态：

```
# FFmpeg 找到
[INFO] FFmpeg found: C:\Users\blycr\bin\ffmpeg.exe
转码引擎: hardware (h264_nvenc) (并发上限: 4)

# FFmpeg 未找到
[WARN] FFmpeg not found (searched: executable dir, ./bin, platform paths, PATH)
转码引擎: unavailable (FFmpeg not found) (并发上限: 0)
```

### 2.4 环境变量

| 变量 | 说明 |
|------|------|
| `MSP_FFMPEG_PATH` | 指定 FFmpeg 可执行文件路径（绝对路径或相对于 CWD） |

---

## 3. 播放策略决策

### 3.1 API 响应

`GET /api/probe` 返回 `playback` 字段（仅在转码配置开启时返回）：

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

### 3.2 决策逻辑

`decidePlaybackMode()` 函数（`internal/handler/stream.go`）基于实际编码判断：

**视频编码**：

| 编码 | 策略 | 原因 |
|------|------|------|
| H.264/AVC | direct | 浏览器原生支持 |
| H.265/HEVC | transcode | Chrome 需硬件解码，Firefox 不支持 |
| AV1 | transcode | 支持不普遍 |
| VC-1/WMV3 | transcode | 浏览器不支持 |
| 未知编码 | transcode | 保守策略 |

**音频编码**：

| 编码 | 策略 | 原因 |
|------|------|------|
| AAC | direct | 浏览器原生支持 |
| MP3 | direct | 浏览器原生支持 |
| Opus | direct | 浏览器原生支持 |
| Vorbis | direct | 浏览器原生支持 |
| FLAC | direct | Chrome 支持，Firefox 部分支持 |
| PCM/LPCM/WAV | direct | 无压缩音频，浏览器支持 |
| AC-3/E-AC-3 | transcode | Chrome 静默跳过，导致无声 |
| DTS/DCA | transcode | 浏览器不支持 |
| TrueHD | transcode | 浏览器不支持 |
| 未知编码 | transcode | 保守策略 |

### 3.3 编码检测方式

编码信息通过两种方式获取：

1. **字节嗅探**（`SniffContainerCodecs`）：读取文件首尾各 2MB，模式匹配
   - MKV：匹配 `V_MPEGH/ISO/HEVC`、`V_MPEG4/ISO/AVC` 等 Matroska 编码标签
   - MP4/M4V/MOV：匹配 `hvc1`、`hev1`、`avc1` 等 FourCC
2. **ffprobe**（`probeCodecInfo`）：精确检测，结果缓存 5 分钟

### 3.4 兼容性设计

编码判断使用 `strings.Contains` 子串匹配，兼容两种标签格式：
- 字节嗅探标签：`"H.264/AVC"`、`"H.265/HEVC"`
- ffprobe 原始名：`"h264"`、`"hevc"`

---

## 4. 硬件加速

### 4.1 支持的编码器

| 模式 | 编码器 | 平台 | 说明 |
|------|--------|------|------|
| `nvenc` | h264_nvenc | 全平台 | NVIDIA GPU |
| `qsv` | h264_qsv | 全平台 | Intel Quick Sync |
| `amf` | h264_amf | Windows | AMD AMF |
| `vaapi` | h264_vaapi | Linux | VA-API |
| `videotoolbox` | h264_videotoolbox | macOS | Apple VideoToolbox |
| `none` | libx264 | 全平台 | 软件编码（禁用硬件加速） |
| `auto` | 自动探测 | 全平台 | 启动时逐一探测，选择最佳 |

### 4.2 探测流程

```
processor.DetectHWAccel(mode)
    │
    ├── mode == "none" → 禁用硬件加速
    │
    ├── 获取平台相关的候选编码器列表
    │       └── hwCandidates() — 按平台过滤
    │
    └── 逐一探测编码器可用性
            └── probeEncoder()
                    ├── 构建测试 FFmpeg 命令
                    ├── 5 秒超时执行
                    └── 成功 → 返回可用编码器
```

### 4.3 并发控制

| 编码类型 | 默认并发上限 | 说明 |
|----------|-------------|------|
| 软件编码 | 2 | 防止 CPU 耗尽 |
| 硬件编码 | 4 | GPU 负载较低，可适当提高 |
| 无 FFmpeg | 0 | 转码功能完全不可用 |

并发通过 buffered channel 信号量控制，转码完成后自动释放。

---

## 5. 智能编码策略

### 5.1 视频转码

```
输入视频
    │
    ├── H.264？ → -vcodec copy（零开销，直接复制流）
    │
    └── 非 H.264？
            │
            ├── 硬件加速可用？
            │       ├── 是 → h264_nvenc/qsv/amf/vaapi/videotoolbox
            │       └── 否 → libx264（软件编码）
            │
            └── 输出 fragmented MP4（支持流式传输）
```

### 5.2 音频转码

```
输入音频
    │
    ├── AAC/MP3？ → -acodec copy（零开销）
    │
    └── 其他？ → 重编码为 AAC
            └── -c:a aac -b:a 192k
```

### 5.3 FFmpeg 参数构建

视频转码的典型 FFmpeg 命令：

```bash
# 硬件加速（NVIDIA）
ffmpeg -hwaccel cuda -i input.mkv \
  -c:v h264_nvenc -preset p4 -tune ll -pix_fmt yuv420p \
  -c:a aac -b:a 192k \
  -movflags +faststart -f mp4 pipe:1

# 软件编码
ffmpeg -i input.mkv \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 192k \
  -movflags +faststart -f mp4 pipe:1

# 视频流复制（H.264 源）
ffmpeg -i input.mp4 \
  -c:v copy \
  -c:a aac -b:a 192k \
  -movflags +faststart -f mp4 pipe:1
```

---

## 6. 配置

### 6.1 转码配置

```json
{
  "playback": {
    "video": {
      "transcode": true,
      "encoding": {
        "hwAccel": "auto",
        "maxJobs": 0
      }
    },
    "audio": {
      "transcode": true
    }
  }
}
```

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `playback.video.transcode` | `true` | 启用视频转码 |
| `playback.audio.transcode` | `true` | 启用音频转码 |
| `playback.video.encoding.hwAccel` | `"auto"` | 硬件加速模式 |
| `playback.video.encoding.maxJobs` | `0` | 最大并发数（0=自动） |

### 6.2 环境变量

| 变量 | 说明 |
|------|------|
| `MSP_FFMPEG_PATH` | 指定 FFmpeg 可执行文件路径 |

---

## 7. 错误处理

### 7.1 前端错误回退

`setupErrorHandler()`（`web/src/modules/player/transcode.js`）作为安全网保留：

1. 浏览器解码错误（code 3/4）
2. 检查是否接近文件末尾（>90%）→ 静默跳到下一个
3. 重试一次（带时间戳刷新 URL）
4. 如果转码配置开启，切换到 `&transcode=1` 源
5. 显示错误提示

正常流程下，前端通过 `getPlaybackUrl()` 直接走正确路径，错误回退极少触发。

### 7.2 后端错误处理

- FFmpeg 进程崩溃 → 返回错误，前端回退到直连
- 转码超时 → 通过 context 取消
- 信号量耗尽 → 返回 503，前端可重试
- 优雅关闭 → `processor.KillAllTranscodeProcesses()` 终止所有活跃进程

---

## 8. 性能优化

### 8.1 CSS 优化（v1.2.0）

- `.panel` / `.now`：移除不可见的 `backdrop-filter`（不透明背景遮挡了模糊效果）
- `.topbar`：模糊半径 12px → 8px，添加 `contain: layout style` + `will-change: transform`
- `.ly`（歌词非活跃行）：`filter: blur(1.5px)` → `opacity: 0.4`

### 8.2 Probe 缓存

- 前端 `probeCache`：Map，500 条上限，重复播放零延迟
- 后端 `probeCache`：`sync.Map` + TTL（5 分钟），避免重复 ffprobe 调用
- 字节嗅探：读取首尾各 2MB，SSD ~1ms，LAN ~5-50ms

---

## 9. 故障排查

### 9.1 FFmpeg 未找到

**症状**：启动日志显示 `unavailable (FFmpeg not found)`

**排查步骤**：
1. 确认 FFmpeg 已安装：`ffmpeg -version`
2. 检查是否在 PATH 中：`where ffmpeg`（Windows）/ `which ffmpeg`（Linux/macOS）
3. 设置环境变量：`MSP_FFMPEG_PATH=/path/to/ffmpeg`
4. 或将 FFmpeg 放在可执行文件同目录

### 9.2 硬件加速不可用

**症状**：启动日志显示 `software (libx264)` 而非 `hardware (h264_nvenc)`

**排查步骤**：
1. 确认 GPU 驱动已安装
2. 手动测试：`ffmpeg -hwaccel cuda -f lavfi -i testsrc=duration=1 -c:v h264_nvenc -f null -`
3. 在配置中指定特定模式：`"hwAccel": "nvenc"`
4. 检查启动日志中的探测结果

### 9.3 转码播放卡顿

**可能原因**：
- 并发转码数过多 → 降低 `maxJobs`
- 硬件编码器性能不足 → 切换到软件编码 `"hwAccel": "none"`
- 网络带宽不足 → 降低 `bitrate`

### 9.4 有画无声

**原因**：音频编码为 AC-3/DTS/TrueHD，浏览器静默跳过

**解决**：确保 `playback.audio.transcode` 为 `true`，后端会自动检测并转码

---

## 10. 相关文件

| 文件 | 职责 |
|------|------|
| `internal/media/probe.go` | FFmpeg 路径发现、ffprobe 调用、编码缓存 |
| `internal/media/transcoder.go` | FFmpeg 转码流处理、并发控制 |
| `internal/media/hwaccel.go` | 硬件加速检测、编码器探测 |
| `internal/handler/stream.go` | 播放策略决策（`decidePlaybackMode`）、流媒体传输 |
| `internal/domain/types.go` | `PlaybackStrategy`、`ProbeResponse` 类型定义 |
| `web/src/modules/player/play.js` | `getPlaybackUrl()` 前端播放策略查询 |
| `web/src/modules/player/transcode.js` | 错误回退兜底 |
| `web/src/app.css` | CSS 性能优化 |
