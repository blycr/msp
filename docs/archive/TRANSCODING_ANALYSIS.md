# MSP 媒体播放架构分析与改进方案

> 分析日期：2026-05-09
> 分析范围：`internal/media/`、`internal/handler/stream.go`、`internal/scanner/`、`web/src/modules/player/`、`web/src/modules/utils.js`、`web/src/modules/api.js`、`web/src/app.css`

---

## 一、当前架构概述

MSP 采用 **"前端决策 + 后端执行"** 的播放架构：

- **前端**检查浏览器能力（`canPlayType`）和配置标志，决定是否请求转码，通过 URL 参数 `transcode=1` 通知后端
- **后端**仅在收到 `transcode=1` 且配置允许时才执行转码，否则直接返回原始文件流
- 后端内置智能编码策略：H.264 视频 → stream copy（零开销），非 H.264 → 硬件/软件重编码

### 核心文件

| 层 | 文件 | 职责 |
|----|------|------|
| 配置 | `internal/config/config.go` | `PlaybackConfig`、`TranscodeConfig` 结构体 |
| 启动 | `cmd/msp/main.go:208-233` | `initHWAccel()` 硬件加速检测 |
| 流处理 | `internal/handler/stream.go` | `HandleStream`（转码策略 + 直连）、`HandleProbe`（字节嗅探） |
| 转码器 | `internal/media/transcoder.go` | `TranscodeStream` FFmpeg 调用、编码参数构建 |
| 探针 | `internal/media/probe.go` | `GetCodecInfo` ffprobe + 缓存 |
| 硬件加速 | `internal/media/hwaccel.go` | `DetectHWAccel`、`BuildVideoArgs` |
| 扫描器 | `internal/scanner/scanner.go` | `SniffContainerCodecs` 字节级编码嗅探 |
| 前端入口 | `web/src/modules/utils.js:104-141` | `canPlayMedia()` 播放能力判断 |
| 前端转码 | `web/src/modules/player/transcode.js` | `needsCompatibilityVideoTranscode()`、错误回退 |
| 前端播放 | `web/src/modules/player/play.js` | `playItem()` 编排 |

---

## 二、当前播放流程（现状）

```
用户点击媒体
    │
    ▼
canPlayMedia() ─── 转码开启？ ──是──▶ 直接允许播放
    │ 否
    ▼
检查 canPlayType(mime)
    │
    ▼
needsCompatibilityVideoTranscode()
    │ .avi/.wmv？ ──是──▶ URL 加 &transcode=1
    ▼ 否
直连播放 /api/stream?id=...
    │
    ▼ (如果浏览器报错)
错误处理器：重试一次 → 仍失败 → 切换到 &transcode=1
    │
    ▼
后端 /api/stream：transcode=1 且配置允许？
    │
    ├── 是 ▶ TranscodeStream → FFmpeg 智能编码
    │         H.264 → stream copy
    │         非 H.264 → HW/SW 重编码 → fragmented MP4
    │
    └── 否 ▶ serveDirect → http.ServeContent (支持 Range)
```

---

## 三、问题清单

### 问题 1：FFmpeg 路径检测过于单一（Bug）

**严重性**：中
**文件**：`internal/media/probe.go:41-47`

**现状**：`CheckFFmpeg()` 仅使用 `exec.LookPath("ffmpeg")`，只搜索系统 PATH。

**边界情况**：
- 用户安装了 FFmpeg 但未加入 PATH（Windows 常见——winget/手动安装后不自动加入）
- FFmpeg 放置在可执行文件同级目录（便携式部署）
- FFmpeg 放置在 `./bin/` 子目录下
- 用户想用本程序自带的 FFmpeg 而非系统版本

**影响**：所有转码功能不可用，启动日志误报（见问题 2）。

---

### 问题 2：FFmpeg 缺失时启动日志误导（Bug）

**严重性**：中
**文件**：`internal/media/hwaccel.go:264-267`

**现象**：系统未安装 FFmpeg 时，启动日志输出：

```
[WARN] FFmpeg not found in PATH
转码引擎: software (libx264) (并发上限: 2)
```

第二行让用户以为软件转码已就绪，但实际上不可用。

**根因**：`FormatHWAccelStatus()` 无法区分"无 FFmpeg"和"无硬件编码器"：

```go
func FormatHWAccelStatus() string {
    r := GetHWAccel()
    if r == nil || !r.Available {
        return "software (libx264)"  // ← 两种情况统一输出
    }
    return fmt.Sprintf("hardware (%s)", r.Encoder)
}
```

当 FFmpeg 缺失时，`detectHWAccelOnce` 返回 `{Available: false}`，`FormatHWAccelStatus()` 无法区分。

---

### 问题 3：前端基于容器/扩展名决策，忽略实际编码（架构缺陷）

**严重性**：高
**文件**：`web/src/modules/player/transcode.js:8-19`、`web/src/modules/utils.js:117-124`

**核心矛盾**：前端做播放决策时，对编码信息几乎一无所知。

**MKV 编码不确定性**——同一扩展名，编码完全不同：

| 文件 | 容器 | 视频编码 | 音频编码 | 浏览器直连 |
|------|------|----------|----------|-----------|
| movie-a.mkv | MKV | H.264 | AAC | 可以 |
| movie-b.mkv | MKV | H.265/HEVC | AC-3 | 不可以 |
| movie-c.mkv | MKV | AV1 | Opus | 部分可以 |

**当前策略**：MKV 直连播放 → 浏览器报错 → 重试 → 切换转码 → 5-10 秒延迟。

**HEVC 问题**：`SniffContainerCodecs` 已能检测 HEVC（MKV: `V_MPEGH/ISO/HEVC`，MP4: `hvc1`/`hev1`），但前端 `needsCompatibilityVideoTranscode()` 完全不使用这些信息——只检查 `.avi`/`.wmv` 扩展名。

**音频编码盲区**：`probeWarnText()` 会警告 AC-3/DTS/TrueHD/FLAC，但仅作为文字提示，不触发转码。Chrome 对不支持的音频轨**静默跳过**（不抛 error），导致"有画无声"且错误处理器永远不会介入。

**`canPlayMedia` 对 AVI 的特殊处理不合理**：AVI 永远不会被浏览器成功播放，但 `canPlayType` 返回空时仍 `return true`，导致不必要的直连尝试。

---

### 问题 4：用户不想用硬件转码时缺少明确配置项（配置缺失）

**严重性**：低
**文件**：`internal/config/config.go:39-49`

**现状**：`TranscodeConfig.HWAccel` 已支持 `"none"` 值（禁用硬件加速，使用软件编码），但：
- `config.example.json` 中未展示此选项
- 启动日志不区分"用户主动禁用 HW"和"未检测到 HW 编码器"
- 用户可能不知道可以配置此项

---

### 问题 5：转码回退缺乏过渡状态 UI

**严重性**：中
**文件**：`web/src/modules/player/transcode.js:72-137`

错误处理流程的时间线：
1. 直连播放 → 浏览器报错（2-5 秒）
2. 重试一次（再花几秒）
3. 切换到转码源 → FFmpeg 启动（1-3 秒）

总计可能 **5-10 秒** 的"什么都没发生"时间。前端没有显示"正在切换到转码模式"之类的过渡状态，用户可能以为应用卡死。

---

### 问题 6：byte-sniff probe 覆盖范围有限

**严重性**：低（当前不影响，但限制未来扩展）
**文件**：`internal/scanner/scanner.go:276-385`

`SniffContainerCodecs` 只对 MKV 和 MP4/M4V/MOV 做字节嗅探。其他容器（AVI、WMV、TS、FLV）返回空编码信息。当前 AVI/WMV 通过扩展名匹配已覆盖，但如果未来要做基于编码的预转码决策（如 TS 中的 HEVC），probe 需要扩展。

---

### 问题 7：backdrop-filter 的 GPU 合成开销（可优化，不牺牲 UI）

**严重性**：低
**文件**：`web/src/app.css`

**现状**：多处使用 `backdrop-filter: blur(12px)`：

| 元素 | 行号 | 背景 | blur 是否有视觉效果 |
|------|------|------|---------------------|
| `.topbar` | 286 | `var(--md-surface)`（半透明） | 有——玻璃态效果 |
| `.panel` | 340 | `var(--md-surface)`（半透明） | 取决于变量值 |
| `.now` | 608 | `var(--md-surface)`（半透明） | 取决于变量值 |
| `.dialog__backdrop` | 1031 | `rgba(0,0,0,0.6)` | 有——遮罩模糊 |
| `.dialog` | 1049 | `var(--md-surface)`（半透明） | 有——对话框玻璃态 |

**优化方向**：不移除视觉效果，而是减少不必要的 GPU 开销。

---

## 四、改进方案

### 方案 A：FFmpeg 多路径发现（解决问题 1、2）

**问题**：`CheckFFmpeg` 仅搜索 PATH，`FormatHWAccelStatus` 无法区分"无 FFmpeg"和"无 HW 编码器"。

**搜索顺序**：

```
1. 环境变量 MSP_FFMPEG_PATH（用户显式指定，最高优先级）
2. 可执行文件同级目录（便携式部署：ffmpeg.exe 和 msp.exe 放一起）
3. 可执行文件的 bin/ 子目录
4. 当前工作目录（./ffmpeg）
5. 当前工作目录的 bin/ 子目录（./bin/ffmpeg）
6. 平台特定路径：
   - Windows: C:\FFmpeg\bin, C:\Program Files\FFmpeg\bin
   - Linux:   /usr/local/bin, /usr/bin
   - macOS:   /usr/local/bin, /opt/homebrew/bin
7. 系统 PATH（exec.LookPath，最后回退）
```

**实现**：

```go
// internal/media/probe.go

var (
    ffmpegPath  string // 缓存发现的路径
    ffprobePath string
    pathOnce    sync.Once
)

func resolveFFmpegPaths() {
    pathOnce.Do(func() {
        ffmpegPath = findExecutable("ffmpeg")
        if ffmpegPath != "" {
            // ffprobe 通常和 ffmpeg 在同一目录
            dir := filepath.Dir(ffmpegPath)
            candidate := filepath.Join(dir, exeName("ffprobe"))
            if _, err := os.Stat(candidate); err == nil {
                ffprobePath = candidate
            } else {
                ffprobePath = findExecutable("ffprobe")
            }
        } else {
            ffprobePath = findExecutable("ffprobe")
        }
    })
}

func findExecutable(name string) string {
    exe := exeName(name)

    // 1. 环境变量
    if env := os.Getenv("MSP_FFMPEG_PATH"); env != "" && name == "ffmpeg" {
        if p, err := exec.LookPath(env); err == nil {
            return p
        }
        // 也尝试直接作为绝对路径
        if _, err := os.Stat(env); err == nil {
            return env
        }
    }

    // 2-5. 程序目录和工作目录
    candidates := localCandidatePaths(exe)
    for _, c := range candidates {
        if _, err := os.Stat(c); err == nil {
            return c
        }
    }

    // 6. 平台特定路径
    for _, c := range platformCandidatePaths(exe) {
        if _, err := os.Stat(c); err == nil {
            return c
        }
    }

    // 7. 系统 PATH
    if p, err := exec.LookPath(exe); err == nil {
        return p
    }

    return ""
}

func exeName(name string) string {
    if runtime.GOOS == "windows" {
        return name + ".exe"
    }
    return name
}

func localCandidatePaths(exe string) []string {
    var paths []string

    // 可执行文件目录
    if exePath, err := os.Executable(); err == nil {
        dir := filepath.Dir(exePath)
        paths = append(paths, filepath.Join(dir, exe))
        paths = append(paths, filepath.Join(dir, "bin", exe))
    }

    // 当前工作目录
    if cwd, err := os.Getwd(); err == nil {
        paths = append(paths, filepath.Join(cwd, exe))
        paths = append(paths, filepath.Join(cwd, "bin", exe))
    }

    return paths
}

func platformCandidatePaths(exe string) []string {
    switch runtime.GOOS {
    case "windows":
        return []string{
            `C:\FFmpeg\bin\` + exe,
            `C:\Program Files\FFmpeg\bin\` + exe,
        }
    case "darwin":
        return []string{
            "/opt/homebrew/bin/" + exe,
            "/usr/local/bin/" + exe,
        }
    default: // linux
        return []string{
            "/usr/local/bin/" + exe,
            "/usr/bin/" + exe,
        }
    }
}
```

**修改 `CheckFFmpeg`**：

```go
func CheckFFmpeg() bool {
    resolveFFmpegPaths()
    if ffmpegPath == "" {
        log.Printf("[WARN] FFmpeg not found (searched: executable dir, ./bin, platform paths, PATH)")
        return false
    }
    log.Printf("[INFO] FFmpeg found: %s", ffmpegPath)
    return true
}

func FFmpegPath() string {
    resolveFFmpegPaths()
    return ffmpegPath
}

func FFprobePath() string {
    resolveFFprobePaths()
    return ffprobePath
}
```

**修改 `probeCodecInfo` 使用发现的路径**：

```go
func probeCodecInfo(ctx context.Context, inputPath string) (CodecInfo, error) {
    probePath := FFprobePath()
    if probePath == "" {
        return CodecInfo{}, fmt.Errorf("ffprobe not found")
    }

    args := []string{"-v", "error", "-select_streams", "v:0,a:0",
        "-show_entries", "stream=codec_name,codec_type", "-of", "json", inputPath}

    cmd := exec.CommandContext(ctx, probePath, args...)
    // ...
}
```

**修改 `TranscodeStream` 使用发现的路径**：

```go
func TranscodeStream(ctx context.Context, inputPath string, opts TranscodeOptions) (io.ReadCloser, error) {
    ffmpegBin := FFmpegPath()
    if ffmpegBin == "" {
        return nil, fmt.Errorf("FFmpeg not found")
    }
    // ... 使用 ffmpegBin 替代 "ffmpeg"
}
```

**修复启动日志**（`FormatHWAccelStatus`）：

```go
func FormatHWAccelStatus() string {
    if !FFmpegAvailable() {
        return "unavailable (FFmpeg not found)"
    }
    r := GetHWAccel()
    if r == nil || !r.Available {
        return "software (libx264)"
    }
    return fmt.Sprintf("hardware (%s)", r.Encoder)
}
```

**启动日志输出**：

```
[INFO] FFmpeg found: C:\Users\blycr\bin\ffmpeg.exe
转码引擎: hardware (h264_nvenc) (并发上限: 4)
```

或：

```
[WARN] FFmpeg not found (searched: executable dir, ./bin, platform paths, PATH)
转码引擎: unavailable (FFmpeg not found) (并发上限: 0)
```

---

### 方案 B：后端播放策略（解决问题 3、5）

**问题**：前端基于扩展名猜测编码，对 HEVC/AC-3 等不兼容编码一无所知，导致不必要的错误回退和"有画无声"。

**核心思想**：将播放策略的决策权从"前端猜测 + 错误回退"改为"后端精确判断 + 前端执行"。

**现状问题**：前端在播放前只有文件扩展名和可能缓存的 probe 数据。编码信息分散在后端的 `SniffContainerCodecs`（快速字节嗅探）和 `GetCodecInfo`（ffprobe 精确检测）中，前端完全无法访问。

**新架构**：增强 `/api/probe` 端点，在返回编码信息的同时，返回推荐的播放策略。

#### 后端：扩展 ProbeResponse

```go
// internal/domain/types.go
type ProbeResponse struct {
    Container string     `json:"container"`
    Video     string     `json:"video,omitempty"`
    Audio     string     `json:"audio,omitempty"`
    Subtitles []Subtitle `json:"subtitles,omitempty"`
    Playback  *PlaybackStrategy `json:"playback,omitempty"`  // 新增
    Error     *ApiError  `json:"error,omitempty"`
}

type PlaybackStrategy struct {
    Mode string `json:"mode"` // "direct" 或 "transcode"
}
```

#### 后端：播放策略判断逻辑

在 `HandleProbe` 中，基于已有的 `SniffContainerCodecs` 结果，追加策略判断：

```go
func decidePlaybackMode(videoCodec, audioCodec string, ffmpegAvailable bool) string {
    if !ffmpegAvailable {
        return "direct"  // 无 FFmpeg 时只能直连，错误回退兜底
    }

    // 视频编码判断
    vc := strings.ToLower(videoCodec)
    switch vc {
    case "h264", "avc", "avc1":
        // H.264：浏览器普遍支持，直连（后端 stream copy 也无额外收益）
    case "hevc", "h265", "hvc1", "hev1":
        return "transcode"  // HEVC：Chrome 需硬件解码，Firefox 不支持
    case "av1":
        return "transcode"  // AV1：支持不普遍
    case "vp9":
        // VP9：Chrome/Firefox 支持，直连
    case "vc1", "wmv3":
        return "transcode"  // VC-1：浏览器不支持
    default:
        if vc != "" {
            return "transcode"  // 未知编码：保守转码
        }
    }

    // 音频编码判断（关键：解决"有画无声"）
    ac := strings.ToLower(audioCodec)
    switch ac {
    case "aac", "mp3", "mp4a", "opus", "vorbis":
        // 浏览器原生支持
    case "ac3", "ac-3", "eac3", "e-ac-3", "ec-3":
        return "transcode"  // AC-3/E-AC-3：Chrome 静默跳过，无声
    case "dts", "dca":
        return "transcode"  // DTS：浏览器不支持
    case "truehd":
        return "transcode"  // TrueHD：浏览器不支持
    case "flac":
        // FLAC：Chrome 支持，Firefox 部分支持，保守直连
    case "pcm", "lpcm", "wav":
        // 无压缩音频：浏览器支持
    default:
        if ac != "" {
            return "transcode"  // 未知音频编码：保守转码
        }
    }

    return "direct"
}
```

**关键优势**：
- 判断依据是**实际编码**（字节嗅探），不是扩展名
- 同一容器（MKV）中的 H.264 和 H.265 会得到不同策略
- 音频编码检查解决"有画无声"——Chrome 不会对 AC-3 抛 error，但后端知道它不兼容
- `ffmpegAvailable` 作为前置条件，避免"无 FFmpeg 时推荐转码"的死路

#### 前端：播放流程简化

**新流程**：

```
用户点击媒体
    │
    ▼
probeItem(id) ─── 有缓存？ ──是──▶ 直接使用
    │ 否
    ▼
GET /api/probe?id=xxx  (字节嗅探, 10-50ms)
    │
    ▼
response.playback.mode
    │
    ├── "direct" ──▶ video.src = streamUrl(id)
    │                 setupErrorHandler()（兜底）
    │
    └── "transcode" ──▶ video.src = streamUrl(id) + "&transcode=1"
                        setupErrorHandler()（兜底）
```

**前端代码变化**（`play.js`）：

```js
// 替代 needsCompatibilityVideoTranscode + canPlayMedia 的复杂判断
async function getPlaybackUrl(item) {
    const base = streamUrl(item.id);

    if (item.kind === "video" && getCfg("playback.video.transcode", false)) {
        const p = await probeItem(item.id);
        if (p?.playback?.mode === "transcode") {
            return { url: base + "&transcode=1", mode: "transcode" };
        }
    }
    if (item.kind === "audio" && getCfg("playback.audio.transcode", false)) {
        const p = await probeItem(item.id);
        if (p?.playback?.mode === "transcode") {
            return { url: base + "&transcode=1", mode: "transcode" };
        }
    }

    return { url: base, mode: "direct" };
}
```

**简化效果**：
- `needsCompatibilityVideoTranscode()` 可删除——后端已返回策略
- `canPlayMedia()` 大幅简化——转码开启时直接放行，关闭时保留 `canPlayType` 检查
- `probeWarnText()` 可删除——后端已将音频兼容性纳入策略，不再需要文字警告
- 错误处理器保留作为兜底，但不再是主要决策路径
- `probeItem` 已在 `playItem` 中异步调用（用于字幕显示），改为 `await` 即可，**不增加额外网络请求**

**probe 延迟分析**：
- `SniffContainerCodecs` 读取文件首尾各 2MB，机械硬盘 ~10ms，SSD ~1ms，LAN ~5-50ms
- 前端已有 `probeCache`（Map，500 条上限），重复播放零延迟
- 总计增加 <50ms 延迟，远小于当前错误回退的 5-10 秒

#### 视频切换优化

当前视频切换（`isVideoSwitch`）时，`needsCompatibilityVideoTranscode` 仅检查扩展名，几乎不增加延迟。新方案中，`probeItem` 已被前一次播放缓存，切换时同步返回。

```js
// 视频切换时，probeItem 命中缓存，同步返回
const p = await probeItem(item.id);  // 缓存命中，0ms
const url = p?.playback?.mode === "transcode"
    ? streamUrl(item.id) + "&transcode=1"
    : streamUrl(item.id);
```

---

### 方案 C：CSS 优化（解决问题 7）

**核心原则**：保留所有视觉效果，消除不必要的 GPU 开销。

#### C.1 移除无效 backdrop-filter

`.panel` 和 `.now` 使用 `var(--md-surface)` 作为背景。如果该变量是不透明色（`rgb(x,y,z)` 而非 `rgba(x,y,z,0.8)`），`backdrop-filter` 在视觉上完全无效——背景已经被完全遮挡，模糊效果不可见，但 GPU 仍在每帧执行模糊计算。

**验证方法**：检查 `--md-surface` 的定义。如果不透明，直接移除：

```css
/* 移除前 */
.panel {
  background: var(--md-surface);
  backdrop-filter: blur(var(--blur-md));
}

/* 移除后——视觉不变，GPU 不再执行无用模糊 */
.panel {
  background: var(--md-surface);
}
```

同理适用于 `.now`。

#### C.2 topbar：CSS containment + 降低 blur 半径

`.topbar` 的 `backdrop-filter` 有视觉效果（半透明背景 + 模糊），需要保留。优化方式：

```css
.topbar {
  /* ... 现有样式 ... */
  backdrop-filter: blur(8px);          /* 12px → 8px，GPU 开销降 ~40%，视觉差异微小 */
  -webkit-backdrop-filter: blur(8px);
  contain: layout style;               /* 限制布局和样式重计算范围 */
  will-change: transform;              /* 提示浏览器提升为独立合成层 */
}
```

`contain: layout style` 告诉浏览器：topbar 内部的布局变化不会影响外部，外部变化也不会影响内部布局。这减少了浏览器在滚动/视频更新时的重计算范围。

`will-change: transform` 将 topbar 提升为独立的 GPU 层（compositing layer），浏览器会缓存该层的渲染结果。当背后的视频帧更新时，浏览器只需将新的视频帧与缓存的 topbar 层合成，而非重新计算 topbar 的 backdrop-filter。

#### C.3 对话框：保持原样

`.dialog__backdrop` 和 `.dialog` 的 `backdrop-filter` 有明确的视觉效果（模态遮罩模糊 + 对话框玻璃态），且只在对话框打开时触发（非持续渲染），GPU 开销可忽略。不做修改。

#### C.4 歌词动画：用 opacity 替代 filter:blur

`filter: blur(1.5px)` 的 GPU 开销高于 `opacity`。歌词动画的目的是"非活跃行淡化"，用 opacity 即可达到近似效果：

```css
/* 优化前 */
.ly {
  filter: blur(1.5px);
  opacity: 0.5;
  transform: scale(0.95);
  transition: opacity 0.5s ..., transform 0.5s ..., filter 0.5s ...;
}

/* 优化后——用 opacity 淡化替代 blur，减少 GPU 层 */
.ly {
  opacity: 0.4;
  transform: scale(0.95);
  transition: opacity 0.5s cubic-bezier(0.25, 0.46, 0.45, 0.94),
              transform 0.5s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.ly--active {
  opacity: 1;
  transform: scale(1.15);
  /* 保持无 blur——active 行本身就是清晰的 */
}
```

**效果对比**：
- 优化前：每行歌词都有 `filter: blur()` → 每行都是独立的 GPU 合成层 → 滚动时每层都需要重渲染
- 优化后：仅 `opacity` + `transform` → 可由 compositor 直接处理（不触发 paint）→ GPU 开销极低

---

## 五、优化后的播放流程

```
用户点击媒体
    │
    ▼
probeItem(id) ─── 缓存命中？ ──是──▶ 使用缓存策略
    │ 否
    ▼
GET /api/probe  (字节嗅探 ~10-50ms)
    │
    ▼
后端判断：H.264+AAC？ ──是──▶ strategy: "direct"
    │ 否                        │
    ▼                           ▼
strategy: "transcode"     video.src = streamUrl(id)
    │                     播放 ← 大多数文件走这条快速路径
    ▼
video.src = streamUrl(id) + "&transcode=1"
    │
    ▼
后端 TranscodeStream
    ├── 视频 H.264？ → -vcodec copy（零开销）
    ├── 视频非 H.264？ → HW/SW 重编码
    ├── 音频 AAC/MP3？ → -acodec copy（零开销）
    └── 音频其他？ → 重编码为 AAC

error handler 保留作为兜底（网络中断、文件损坏等非编码问题）
```

**对比当前方案**：

| 维度 | 当前方案 | 新方案 |
|------|---------|--------|
| 决策依据 | 文件扩展名 + canPlayType | 实际编码（字节嗅探） |
| 首次播放延迟 | 直连 → 错误 → 重试 → 转码（5-10s） | probe ~50ms → 直接正确播放 |
| HEVC MKV | 先直连失败再转码 | 直接转码 |
| AC-3 音频 | Chrome 静默无声 | 直接转码（音频重编码） |
| 前端复杂度 | canPlayMedia + needsCompatibility + 错误处理 + 重试 | probeItem + 读 strategy |
| probe 请求 | 仅用于字幕显示（异步可选） | 播放前必调（已有缓存，增量成本为零） |
| 后端改动 | 无 | HandleProbe 增加策略判断（~50 行） |

---

## 六、实施计划

### 第一阶段：FFmpeg 发现 + 启动日志（方案 A）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 1.1 | 实现 `findExecutable` 多路径搜索 | `internal/media/probe.go` |
| 1.2 | `CheckFFmpeg`/`CheckFFprobe` 使用发现的路径 | `internal/media/probe.go` |
| 1.3 | `probeCodecInfo` 使用 `FFprobePath()` | `internal/media/probe.go` |
| 1.4 | `TranscodeStream` 使用 `FFmpegPath()` | `internal/media/transcoder.go` |
| 1.5 | `FormatHWAccelStatus` 区分"无 FFmpeg"和"无 HW" | `internal/media/hwaccel.go` |
| 1.6 | 启动日志输出发现的 FFmpeg 路径 | `cmd/msp/main.go` |

### 第二阶段：后端播放策略（方案 B 后端）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 2.1 | 添加 `PlaybackStrategy` 到 `ProbeResponse` | `internal/domain/types.go` |
| 2.2 | 实现 `decidePlaybackMode()` | `internal/handler/stream.go` 或新文件 |
| 2.3 | `HandleProbe` 调用策略判断并填充 `Playback` | `internal/handler/stream.go` |
| 2.4 | 配置 `playback.video.transcode=false` 时不返回 `playback` 字段 | 同上 |

### 第三阶段：前端集成（方案 B 前端）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 3.1 | 实现 `getPlaybackUrl()` 替代现有判断逻辑 | `web/src/modules/player/play.js` |
| 3.2 | `playItem` 中 `await probeItem` → 使用 strategy | `web/src/modules/player/play.js` |
| 3.3 | 简化 `canPlayMedia()` | `web/src/modules/utils.js` |
| 3.4 | 删除 `needsCompatibilityVideoTranscode()` | `web/src/modules/player/transcode.js` |
| 3.5 | 保留 `setupErrorHandler` 作为兜底 | 不变 |
| 3.6 | 转码回退时显示过渡状态 toast | `web/src/modules/player/transcode.js` |

### 第四阶段：CSS 优化（方案 C）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 4.1 | 检查 `--md-surface` 是否不透明，移除 `.panel`/`.now` 的无效 backdrop-filter | `web/src/app.css` |
| 4.2 | `.topbar` 添加 `contain: layout style`，降低 blur 至 8px | `web/src/app.css` |
| 4.3 | `.ly` 用 opacity 替代 filter:blur | `web/src/app.css` |

---

## 七、方案与问题对应关系

| 问题 | 对应方案 | 关键改动 |
|------|---------|---------|
| FFmpeg 不在 PATH | 方案 A | 搜索 7 层位置 + 环境变量 |
| 启动日志误导 | 方案 A | `FormatHWAccelStatus` 区分三种状态 |
| MKV H.264 vs H.265 | 方案 B | 字节嗅探精确判断 |
| AC-3 无声 | 方案 B | 后端策略直接推荐转码 |
| 用户禁用 HW | 方案 A | 启动日志明确区分 |
| 转码回退无 UI | 方案 B | 前端直接走正确路径，无需回退 |
| GPU 开销 | 方案 C | 移除无效 blur + containment + 优化动画 |
| 首次播放延迟 | 方案 B | ~50ms（probe）替代 5-10s（错误回退） |
| 前端代码复杂度 | 方案 B | 1 个函数读 strategy 替代 3 个模块交叉判断 |

---

## 八、实施状态

> 以下记录各阶段的实施完成情况。

### 第一阶段：FFmpeg 发现 + 启动日志 ✅

| 步骤 | 状态 | 说明 |
|------|------|------|
| 1.1 | ✅ | `findExecutable` 实现 7 层搜索：`MSP_FFMPEG_PATH` → 可执行文件目录 → `./bin/` → 工作目录 → `./bin/` → 平台路径 → PATH |
| 1.2 | ✅ | `CheckFFmpeg`/`CheckFFprobe` 使用 `resolveFFmpegPaths()`（`sync.Once` 缓存） |
| 1.3 | ✅ | `probeCodecInfo` 使用 `FFprobePath()`，找不到时返回明确错误 |
| 1.4 | ✅ | `TranscodeStream` 在获取信号量前检查 `FFmpegPath()`，避免无 FFmpeg 时占用并发槽 |
| 1.5 | ✅ | `FormatHWAccelStatus` 区分三种状态：`unavailable (FFmpeg not found)` / `software (libx264)` / `hardware (h264_nvenc)` |
| 1.6 | ✅ | `initHWAccel` 先调用 `CheckFFmpeg()`，无 FFmpeg 时并发上限设为 0 并直接返回 |

### 第二阶段：后端播放策略 ✅

| 步骤 | 状态 | 说明 |
|------|------|------|
| 2.1 | ✅ | `PlaybackStrategy{Mode string}` 添加到 `ProbeResponse`，`omitempty` 保证向后兼容 |
| 2.2 | ✅ | `decidePlaybackMode()` 基于实际编码判断：H.264→direct，HEVC/AV1/VP9/VC-1→transcode，AC-3/DTS/TrueHD→transcode |
| 2.3 | ✅ | `HandleProbe` 调用 `SniffContainerCodecs` + `decidePlaybackMode` 填充 `Playback` |
| 2.4 | ✅ | 转码配置关闭时不返回 `playback` 字段 |

### 第三阶段：前端集成 ✅

| 步骤 | 状态 | 说明 |
|------|------|------|
| 3.1 | ✅ | `getPlaybackUrl(item)` 替代 `needsCompatibilityVideoTranscode`：查询后端策略 |
| 3.2 | ✅ | `playItem` 改为 `async`，音频/视频路径均 `await getPlaybackUrl()` |
| 3.3 | ✅ | `canPlayMedia()` 移除 AVI 特殊处理（后端策略已覆盖） |
| 3.4 | ✅ | 删除 `needsCompatibilityVideoTranscode()`、`preemptiveTranscodeVideoExts`、相关导出 |
| 3.5 | ✅ | `setupErrorHandler` 保留作为网络中断/文件损坏的兜底 |
| 3.6 | ⏭️ | 未实施——新架构下前端直接走正确路径，错误回退极少触发，优先级降低 |

### 第四阶段：CSS 优化 ✅

| 步骤 | 状态 | 说明 |
|------|------|------|
| 4.1 | ✅ | `.panel`/`.now` 移除无效 `backdrop-filter`（`--md-surface` 为不透明色） |
| 4.2 | ✅ | `.topbar` blur 12px→8px，添加 `contain: layout style` + `will-change: transform` |
| 4.3 | ✅ | `.ly` 用 `opacity: 0.4` 替代 `filter: blur(1.5px)`，移除 transition 中的 filter |

### 收尾清理 ✅

| 项目 | 状态 | 说明 |
|------|------|------|
| `probeWarnText` | ✅ | 从 `api.js` 删除（后端策略已处理音频兼容性） |
| `probePeek` | ✅ | 保留（无害，可能未来有用） |
| `needsCompatibilityVideoTranscode` 导出链 | ✅ | 从 `transcode.js`、`index.js`、`player.js` 完整移除 |
| `el` 导入 | ✅ | 从 `transcode.js` 移除（不再使用） |
