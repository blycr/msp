# MSP 进化实施计划

> **目标读者**：执行此计划的 AI。请逐特性实施，每完成一个特性确保 `go test ./...` 和 `cd web && bun run build` 通过后再进入下一个。
>
> **兜底原则**：如果某个步骤的具体实现方案在实际代码中行不通（文件结构变化、API 签名不匹配等），你有权自由调整实现方式，只要最终效果一致。遇到不确定的情况，优先选择最小改动的方案。

---

## 全局规则

1. **构建验证**：每个特性完成后运行 `cd web && bun run build && cd .. && go test ./... && go vet ./...`
2. **代码风格**：Go 遵循现有 handler 模式（writeJSON/writeError）；JS 遵循现有 EventBus 模式
3. **i18n**：所有用户可见文案必须同时添加 en/zh 翻译到 `web/src/modules/i18n.js`
4. **测试**：后端改动必须有对应单元测试；前端无测试框架，不要求测试
5. **版本**：所有改动归入下一个版本（在 CHANGELOG.md 中记录）

---

## 特性 1：继续观看（Continue Watching）

**场景**：用户打开 MSP 首页，立刻看到上次看到一半的视频/音频，一键继续。
**复杂度**：Light — 纯利用现有 `PlaybackProgress` 数据，无需新建表。

### 1.1 后端：新增 `/api/progress/recent` 端点

**文件**：`internal/handler/progress.go`

在 `HandleProgress` 函数之后，新增 `HandleRecentProgress`：

```go
// HandleRecentProgress returns recently played media with progress.
// GET /api/progress/recent?limit=10
func (h *Handler) HandleRecentProgress(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
    limitStr := r.URL.Query().Get("limit")
    limit := 10
    if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 50 {
        limit = v
    }
    items, err := h.progress.ListRecentProgress(r.Context(), limit)
    if err != nil {
        log.Printf("Error in ListRecentProgress: %v", err)
        writeError(w, http.StatusInternalServerError, "failed to list recent progress")
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
```

> **注意**：需要在文件顶部的 import 中添加 `"strconv"`（如果尚未导入）。

**文件**：`cmd/msp/main.go` — 注册路由

在 `mux.Handle("/api/progress", ...)` 之后添加：
```go
mux.Handle("/api/progress/recent", http.HandlerFunc(h.HandleRecentProgress))
```

> **兜底**：如果路由注册的位置或方式有变化，找到其他 `/api/` 路由注册的地方，按照同样的模式添加。

### 1.2 后端：存储层新增方法

**文件**：`internal/storage/interface.go`

在 `ProgressStore` 接口中新增方法：
```go
type ProgressStore interface {
    GetProgress(ctx context.Context, mediaID string) (float64, error)
    SetProgress(ctx context.Context, mediaID string, t float64) error
    ListRecentProgress(ctx context.Context, limit int) ([]domain.PlaybackProgress, error)
}
```

**文件**：`internal/storage/sqlite.go`

新增实现：
```go
func (s *SQLite) ListRecentProgress(ctx context.Context, limit int) ([]domain.PlaybackProgress, error) {
    dbConn, ok := s.guard("ListRecentProgress")
    if !ok {
        return nil, nil
    }
    if limit <= 0 {
        limit = 10
    }
    var items []domain.PlaybackProgress
    err := dbConn.WithContext(ctx).
        Order("updated_at DESC").
        Limit(limit).
        Find(&items).Error
    return items, err
}
```

> **兜底**：如果 `PlaybackProgress` 没有 `updated_at` 字段，需要先在 `domain/types.go` 的 `PlaybackProgress` 结构体中添加 `UpdatedAt time.Time` 字段（带 gorm tag `gorm:"autoUpdateTime"`）。GORM 的 AutoMigrate 会自动添加列。检查现有结构体再决定。

### 1.3 后端：确保 PlaybackProgress 有时间戳

**文件**：`internal/domain/types.go`

检查 `PlaybackProgress` 结构体。当前结构大致是：
```go
type PlaybackProgress struct {
    MediaID   string    `json:"mediaId" gorm:"primaryKey;column:media_id"`
    Time      float64   `json:"time"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

如果没有 `UpdatedAt`，添加：
```go
UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
```

> **兜底**：如果已有 `UpdatedAt`，跳过此步。关键是 `ListRecentProgress` 能按更新时间降序排列。

### 1.4 后端测试

**文件**：`internal/handler/progress_test.go`（或新建）

添加测试：
```go
func TestHandleRecentProgress(t *testing.T) {
    // 使用已有的 mock 模式（参考其他 handler 测试文件如 handler_test.go）
    // 测试：
    // 1. GET 返回 200 + JSON array
    // 2. limit 参数生效
    // 3. 非 GET 返回 405
}
```

**文件**：`internal/storage/sqlite_test.go`（如无则参考已有 storage 测试）

```go
func TestListRecentProgress(t *testing.T) {
    // 创建内存 SQLite，写入 3 条 progress，验证按时间降序返回
}
```

### 1.5 前端：启动时加载继续观看数据

**文件**：`web/src/modules/api.js`

新增函数：
```js
export async function loadRecentProgress(limit = 5) {
  return apiGet(`/api/progress/recent?limit=${limit}`);
}
```

### 1.6 前端：渲染"继续观看"区域

**文件**：`web/src/modules/ui/render.js`

在 `renderList()` 函数开头（清空 `box.innerHTML` 之后，渲染文件列表之前），插入继续观看区域的渲染逻辑：

```js
// 在 renderList 中，box.innerHTML = "" 之后加入：
function renderContinueWatching(box) {
  if (!state.continueWatching || state.continueWatching.length === 0) return;
  if (state.q) return; // 搜索模式下不显示

  const section = document.createElement('div');
  section.className = 'continue-watching';
  section.innerHTML = `<div class="continue-watching__title">${t('continue_watching')}</div>`;

  const list = document.createElement('div');
  list.className = 'continue-watching__list';

  for (const item of state.continueWatching) {
    const el = document.createElement('button');
    el.className = 'continue-watching__item';
    el.dataset.id = item.id;
    el.innerHTML = `
      <span class="continue-watching__name">${formatName(item.name)}</span>
      <span class="continue-watching__progress">${formatTime(item.time)}</span>
    `;
    el.addEventListener('click', () => {
      bus.emit('play:request', item, { resume: true });
    });
    list.appendChild(el);
  }
  section.appendChild(list);
  box.prepend(section);
}
```

> **兜底**：上面 `item.name` 需要从媒体列表中匹配。因为 `/api/progress/recent` 只返回 `{mediaId, time}`，需要在前端与 `state.media` 做 join。见 1.7。

### 1.7 前端：数据匹配逻辑

**文件**：`web/src/modules/actions.js`

在 `boot()` 函数中，`loadMedia` 完成后加载继续观看数据：

```js
// 在 "Full load in background" 的 setTimeout 回调末尾添加：
import { loadRecentProgress } from './api.js';

// 在 loadMedia 完成后：
try {
  const progressData = await loadRecentProgress(5);
  if (progressData?.items?.length && state.media) {
    const allMedia = [
      ...(state.media.videos || []),
      ...(state.media.audios || []),
    ];
    state.continueWatching = progressData.items
      .map(p => {
        const media = allMedia.find(m => m.id === p.mediaId);
        if (!media) return null;
        return { ...media, time: p.time };
      })
      .filter(Boolean);
    renderList(); // 触发重新渲染
  }
} catch (e) {
  console.warn("Failed to load continue watching:", e);
}
```

> **兜底**：`import` 语句要放到文件顶部。`renderList` 如果不在 actions.js 的 import 中，通过 bus 事件触发：`bus.emit('list:render')`，或直接 import。参考已有 import 关系决定。

### 1.8 前端：状态扩展

**文件**：`web/src/modules/state.js`

在 `state` 对象中添加：
```js
continueWatching: [], // Array of {id, name, kind, time, ...MediaItem}
```

### 1.9 前端：样式

**文件**：`web/src/styles/components/list.css`（在文件末尾追加）

```css
/* Continue Watching */
.continue-watching {
  padding: 8px 0 12px;
  border-bottom: 1px solid var(--md-border);
  margin-bottom: 8px;
}
.continue-watching__title {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
  padding: 0 12px 6px;
}
.continue-watching__list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.continue-watching__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  border-radius: var(--radius-sm, 4px);
  transition: background 0.15s;
  font: inherit;
  color: var(--md-on-surface);
  width: 100%;
}
.continue-watching__item:hover {
  background: var(--md-surface-variant);
}
.continue-watching__name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}
.continue-watching__progress {
  font-size: 11px;
  color: var(--md-primary);
  margin-left: 8px;
  flex-shrink: 0;
}
```

### 1.10 前端：i18n

**文件**：`web/src/modules/i18n.js`

在 `en` 对象中添加：
```js
continue_watching: "Continue Watching",
```

在 `zh` 对象中添加：
```js
continue_watching: "继续观看",
```

### 1.11 验证清单

- [ ] `go test ./...` 通过
- [ ] `cd web && bun run build` 通过
- [ ] 启动后，先播放一个视频/音频并暂停，刷新页面，侧边栏顶部显示"继续观看"
- [ ] 点击继续观看条目能跳转播放并恢复进度
- [ ] 搜索模式下不显示继续观看区域

---

## 特性 2：视频缩略图

**场景**：视频列表从纯文件名变为带缩略图的视觉化列表，用户一眼识别内容。
**复杂度**：Standard — 需要后端 ffmpeg 截图 + 缓存 + 前端渲染改动。
**前置条件**：系统安装了 ffmpeg。

### 2.1 后端：新增缩略图端点

**文件**：新建 `internal/handler/thumbnail.go`

```go
package handler

import (
    "crypto/sha256"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "sync"

    "msp/internal/constants"
)

var thumbLock sync.Mutex
var thumbSema = make(chan struct{}, 2) // 最多同时生成 2 张缩略图

// HandleThumbnail serves video thumbnails.
// GET /api/thumbnail?id=xxx
func (h *Handler) HandleThumbnail(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    id := r.URL.Query().Get("id")
    if id == "" {
        writeError(w, http.StatusBadRequest, constants.ErrMsgMissingID)
        return
    }

    filePath, err := h.idCodec.DecodeID(id)
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid id")
        return
    }

    // 检查文件是否存在
    if _, err := os.Stat(filePath); err != nil {
        writeError(w, http.StatusNotFound, "file not found")
        return
    }

    // 缩略图缓存目录
    thumbDir := filepath.Join(filepath.Dir(filePath), ".msp_thumbs")
    hash := fmt.Sprintf("%x", sha256.Sum256([]byte(filePath)))
    thumbPath := filepath.Join(thumbDir, hash+".jpg")

    // 检查缓存
    if info, err := os.Stat(thumbPath); err == nil && info.Size() > 0 {
        w.Header().Set("Cache-Control", "public, max-age=86400")
        http.ServeFile(w, r, thumbPath)
        return
    }

    // 检查 ffmpeg 可用性
    if !h.processor.FFmpegAvailable() {
        writeError(w, http.StatusServiceUnavailable, "ffmpeg not available")
        return
    }

    // 限制并发
    select {
    case thumbSema <- struct{}{}:
        defer func() { <-thumbSema }()
    default:
        writeError(w, http.StatusTooManyRequests, "thumbnail generation busy")
        return
    }

    // 生成缩略图
    if err := os.MkdirAll(thumbDir, 0755); err != nil {
        log.Printf("[WARN] thumbnail: mkdir failed: %v", err)
        writeError(w, http.StatusInternalServerError, "failed to create cache dir")
        return
    }

    ffmpegPath := h.processor.FFmpegPath()
    cmd := exec.CommandContext(r.Context(), ffmpegPath,
        "-ss", "5",           // 跳过前 5 秒
        "-i", filePath,
        "-vframes", "1",
        "-vf", "scale=320:-1", // 宽 320px，高度等比
        "-q:v", "8",           // JPEG 质量
        "-y",
        thumbPath,
    )
    if output, err := cmd.CombinedOutput(); err != nil {
        log.Printf("[WARN] thumbnail: ffmpeg failed for %s: %v\n%s", filePath, err, output)
        writeError(w, http.StatusInternalServerError, "thumbnail generation failed")
        return
    }

    w.Header().Set("Cache-Control", "public, max-age=86400")
    http.ServeFile(w, r, thumbPath)
}
```

> **兜底注意事项**：
> 1. `h.processor.FFmpegAvailable()` 和 `h.processor.FFmpegPath()` 可能不存在。检查 `internal/media/processor.go` 中 `MediaProcessor` 的方法。如果没有，需要添加简单的 getter 方法，或者直接用 `exec.LookPath("ffmpeg")` 替代。
> 2. 缩略图目录用 `.msp_thumbs` 放在视频同目录下。如果权限不允许，改为放在 exe 目录下的 `thumbs/` 子目录中（`filepath.Join(util.MustExeDir(), "thumbs", hash+".jpg")`）。
> 3. `constants.ErrMsgMissingID` 已存在于项目中。

### 2.2 后端：暴露 FFmpeg 路径方法

**文件**：`internal/media/processor.go`

检查是否已有 `FFmpegPath()` 和 `FFmpegAvailable()` 方法。如果没有，添加：

```go
func (mp *MediaProcessor) FFmpegPath() string {
    if mp == nil { return "" }
    mp.initOnce.Do(mp.init)
    return mp.ffmpegPath
}

func (mp *MediaProcessor) FFmpegAvailable() bool {
    return mp.FFmpegPath() != ""
}
```

> **兜底**：`MediaProcessor` 内部字段名可能不同（如 `ffmpegBin`）。查看实际字段名并适配。如果已有类似方法（如 `GetFFmpegPath`），直接使用。

### 2.3 后端：注册路由

**文件**：`cmd/msp/main.go`

```go
mux.Handle("/api/thumbnail", http.HandlerFunc(h.HandleThumbnail))
```

### 2.4 前端：列表项增加缩略图

**文件**：`web/src/modules/ui/render.js`

找到渲染文件列表项的代码（`renderList` 函数中构造 HTML 的部分），为 video 类型的 item 添加缩略图 `<img>`：

```js
// 在构造文件项 HTML 时，对 kind === "video" 的项目，添加：
const thumbHtml = item.kind === "video"
  ? `<img class="file-thumb" src="/api/thumbnail?id=${encodeURIComponent(item.id)}" loading="lazy" alt="" />`
  : '';
```

将这个 `thumbHtml` 插入到文件项的 HTML 模板中（在文件名之前）。

> **兜底**：`renderList` 的具体 HTML 构建方式需要查看实际代码。可能是 `innerHTML` 模板字符串，也可能是 `createElement`。按照实际方式适配。关键是在每个 video 类型的列表项前加一个 `<img>` 标签。

### 2.5 前端：缩略图样式

**文件**：`web/src/styles/components/list.css`（追加）

```css
/* Video Thumbnails */
.file-thumb {
  width: 48px;
  height: 32px;
  object-fit: cover;
  border-radius: 3px;
  flex-shrink: 0;
  background: var(--md-surface-variant);
  margin-right: 8px;
}
.file-thumb[src=""],
.file-thumb:not([src]) {
  display: none;
}
```

### 2.6 后端测试

**文件**：新建 `internal/handler/thumbnail_test.go`

```go
func TestHandleThumbnail_MissingID(t *testing.T) {
    // 测试无 id 参数返回 400
}

func TestHandleThumbnail_InvalidMethod(t *testing.T) {
    // 测试 POST 返回 405
}

func TestHandleThumbnail_InvalidID(t *testing.T) {
    // 测试无效 id 返回 400
}
```

### 2.7 验证清单

- [ ] `go test ./...` 通过
- [ ] `cd web && bun run build` 通过
- [ ] 启动后，视频列表项左侧显示缩略图
- [ ] 首次加载缩略图后，第二次加载从缓存返回（检查 Cache-Control header）
- [ ] ffmpeg 不可用时，缩略图位置隐藏，不影响正常功能
- [ ] 同时打开多个视频不会因并发限制导致崩溃

---

## 特性 3：转码进度反馈

**场景**：用户点击播放需要转码的视频时，看到"转码准备中..."提示，而不是一片空白等待。
**复杂度**：Light — 纯前端改动 + 利用已有 probe 信息。

### 3.1 前端：播放时显示转码状态

**文件**：`web/src/modules/player/play.js`

在 `playItem` 函数中，`getPlaybackUrl` 调用前后添加状态提示：

找到 `getPlaybackUrl` 的调用位置，在其前面添加：
```js
// 显示转码提示
if (item.kind === "video" || item.kind === "audio") {
  bus.emit('transcode:status', 'checking');
}
```

在 `getPlaybackUrl` 返回后：
```js
if (playback.mode === "transcode") {
  bus.emit('transcode:status', 'transcoding');
} else {
  bus.emit('transcode:status', null);
}
```

在媒体元素的 `canplay` 事件中清除提示：
```js
mediaEl.addEventListener('canplay', () => {
  bus.emit('transcode:status', null);
}, { once: true });
```

> **兜底**：`playItem` 函数的具体结构需要查看实际代码。关键是在请求播放 URL 前显示提示、获取到 transcode 模式后显示"转码中"、媒体可播放后隐藏提示。

### 3.2 前端：转码状态 UI 组件

**文件**：`web/src/modules/ui/render.js`（或新建 `web/src/modules/ui/transcode-status.js`）

```js
import { bus } from '../eventbus.js';
import { el } from '../state.js';
import { t } from '../i18n.js';

bus.on('transcode:status', (status) => {
  let indicator = document.getElementById('transcodeStatus');
  if (!indicator) {
    indicator = document.createElement('div');
    indicator.id = 'transcodeStatus';
    indicator.className = 'transcode-status';
    const playerBox = el('playerBox');
    if (playerBox) playerBox.prepend(indicator);
  }

  if (!status) {
    indicator.hidden = true;
    return;
  }

  indicator.hidden = false;
  if (status === 'checking') {
    indicator.textContent = t('transcode_checking');
  } else if (status === 'transcoding') {
    indicator.innerHTML = `<span class="transcode-status__spinner"></span>${t('transcode_preparing')}`;
  }
});
```

> **兜底**：如果更倾向于在 `ui/index.js` 中注册 bus 监听，也可以。关键是监听 `transcode:status` 事件并更新 UI。

### 3.3 前端：样式

**文件**：`web/src/styles/components/player.css`（追加）

```css
/* Transcode Status */
.transcode-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  font-size: 12px;
  color: var(--md-on-surface-variant);
  background: var(--md-surface-variant);
  border-radius: var(--radius-sm, 4px);
  margin-bottom: 8px;
}
.transcode-status[hidden] { display: none; }
.transcode-status__spinner {
  width: 14px; height: 14px;
  border: 2px solid var(--md-border);
  border-top-color: var(--md-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
```

### 3.4 前端：i18n

**文件**：`web/src/modules/i18n.js`

en:
```js
transcode_checking: "Checking compatibility...",
transcode_preparing: "Transcoding video, please wait...",
```

zh:
```js
transcode_checking: "正在检测兼容性...",
transcode_preparing: "正在转码，请稍候...",
```

### 3.5 验证清单

- [ ] `cd web && bun run build` 通过
- [ ] 播放需要转码的视频时，播放器上方显示"正在转码"提示 + 旋转动画
- [ ] 视频可以播放后，提示自动消失
- [ ] 直接播放的视频不显示提示
- [ ] 音频转码同样显示提示

---

## 执行顺序与版本规划

```
v1.7.0
├── 特性 1: 继续观看 (Continue Watching)
├── 特性 3: 转码进度反馈 (Transcode Status Feedback)
└── 特性 2: 视频缩略图 (Video Thumbnails)
```

**推荐执行顺序**：1 → 3 → 2（从简到难）

每个特性完成后：
1. 运行构建验证命令
2. 手动测试一遍验证清单
3. 在 CHANGELOG.md 中记录改动

---

## 安全兜底策略

### 如果遇到编译错误
1. 先看错误信息，90% 是接口不匹配
2. 检查被修改的接口的所有实现（`grep -rn "接口方法名" internal/`）
3. 确保 mock 实现也更新了

### 如果遇到前端构建错误
1. 通常是 import 路径错误或循环依赖
2. MSP 前端使用 EventBus 解耦，如果出现循环依赖，改用 `bus.emit/on` 替代直接 import

### 如果某个特性的方案不可行
1. 记录具体原因
2. 跳过该特性，继续下一个
3. 在 CHANGELOG 中标注 "deferred: [原因]"

### 关键约束
- **不要修改** `internal/storage/interface.go` 中已有方法的签名
- **不要修改** `internal/handler/handler.go` 中 `Deps` 结构体的已有字段
- **不要修改** 已有的 CSS class 名称（会影响现有样式）
- 新增的 CSS 变量如果不存在（如 `--radius-sm`），用固定值替代（如 `4px`）
