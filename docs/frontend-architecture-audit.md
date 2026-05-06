# 前端模块架构审计报告

> 生成日期：2026-05-06
> 审计范围：`web/src/modules/`

## 总览

| 类别 | 数量 | 严重程度 |
|------|------|----------|
| 代码重复 | 5 处 | 🔴 高 |
| 循环依赖 | 2 条 | 🟡 中 |
| 死代码 / 未使用导出 | 7 处 | 🟡 中 |
| 无监听者的 bus 事件 | 2 个 | 🟠 中低 |
| 架构模式不一致 | 3 处 | 🟡 中 |

---

## 1. 代码重复（最严重的问题）

### 1.1 `renderPlaylist()` — 两份完整实现

| 位置 | 行数 | 说明 |
|------|------|------|
| `playlist.js` L90-175 | ~85 行 | 手动 DOM 构建，含分页 |
| `playlist/render.js` L140-227 | ~87 行 | 几乎完全相同的 DOM 构建 + 分页 + `scheduleAutoFitPlaylistPageSize()` |

**差异**：`playlist.js` 版本在末尾也调用了 `scheduleAutoFitPlaylistPageSize()`，但调用的是 `playlist.js` 自身重新包装的版本。两个版本在 `playAtIndex` 绑定上有细微差别（`playlist.js` 版本不传 `user` 参数，`playlist/render.js` 版本传 `true`）。

**风险**：两份实现可能逐渐分叉，造成行为不一致。

**建议**：删除 `playlist.js` 中的 `renderPlaylist()`，改为只使用 `playlist/render.js` 导出的版本。

### 1.2 `playAtIndex()` — 两份实现，行为不一致

| 位置 | 差异 |
|------|------|
| `playlist.js` L57-72 | 调用 `renderPlaylist()` + `updateNavButtons()`，然后 `bus.emit('play:request', ...)` |
| `playlist/navigation.js` L39-54 | 调用 `bus.emit('playlist:updated')` + `updateNavButtons()`，然后 `bus.emit('play:request', ...)` |

**关键差异**：
- `playlist.js` 版本调用 `renderPlaylist()` 但 **不** 发射 `playlist:updated` 事件
- `navigation.js` 版本发射 `playlist:updated` 事件但 **不** 调用 `renderPlaylist()`

**风险**：调用路径不同可能导致播放列表 UI 刷新行为不一致。

**建议**：只保留 `playlist/navigation.js` 的版本，删除 `playlist.js` 中的重复实现。如果需要 `renderPlaylist()` 调用，应该在 `play:request` 的监听器中统一处理。

### 1.3 `resumeLast()` — 两份几乎相同的实现

| 位置 | 行数 |
|------|------|
| `player.js` L66-125 | ~60 行 |
| `player/resume.js` L15-74 | ~60 行 |

两份代码逻辑几乎完全相同（恢复上次播放的媒体、恢复音量、恢复播放进度）。唯一区别是 `player/resume.js` 版本多了一行 `rememberEnabled()` 函数（本地定义），而 `player.js` 版本从 `api.js` 导入 `rememberEnabled`。

**风险**：修改一处不会自动同步到另一处，容易导致行为分叉。

**建议**：删除 `player.js` 中的 `resumeLast()`，只保留 `player/resume.js` 的版本，并通过 `player/index.js` 导出。

### 1.4 `bindGlobalHotkeys()` — 两份几乎相同的实现

| 位置 | 行数 |
|------|------|
| `player.js` L485-579 | ~95 行 |
| `player/resume.js` L76-178 | ~103 行 |

两份代码完全相同（键盘快捷键：空格播放/暂停、方向键快进快退/音量、M 静音、[] 切歌、F 全屏）。

**建议**：删除 `player.js` 中的 `bindGlobalHotkeys()`，只保留 `player/resume.js` 的版本。

### 1.5 `rememberEnabled()` — 三份相同实现

| 位置 | 类型 |
|------|------|
| `api.js` L146 | `export function` |
| `player/resume.js` L9 | `function`（本地） |
| `player/seek.js` L66 | `function`（本地） |

三份代码完全相同：
```js
function rememberEnabled(kind) {
  const cfg = state.config?.playback?.[kind];
  if (!cfg) return true;
  return cfg.remember !== false;
}
```

**建议**：只保留 `api.js` 中的导出版本，其他两处改为从 `api.js` 导入。

### 1.6 `canStorage()` — 两份相同实现

| 位置 | 类型 |
|------|------|
| `state.js` L103 | `export function` |
| `player/core.js` L106 | `export function`（通过 `player/index.js` 导出） |

**建议**：只保留 `state.js` 中的版本，`player/core.js` 改为从 `state.js` 导入。

---

## 2. 循环依赖

### 2.1 `actions.js → ui.js → ui/bindings.js → actions.js`

```
actions.js  ──import('./ui.js')──→  ui.js
                                       │
                                  import from ui/bindings.js
                                       │
                                       ▼
                                 ui/bindings.js
                                       │
                                  import from actions.js (loadConfig, loadMedia)
                                       │
                                       ▼
                                  actions.js  ← 循环！
```

**当前状态**：ESM 的 live bindings 使得这个循环在运行时能工作（`bus.on()` 注册回调时只需要函数引用，回调执行时所有模块已完全加载），但这是脆弱的。

**建议**：将 `loadConfig` 和 `loadMedia` 通过 bus 事件暴露（如 `bus.emit('config:reload')`、`bus.emit('media:refresh')`），而不是直接导入。这样 `ui/bindings.js` 就不需要导入 `actions.js`。

### 2.2 `player.js ↔ player/resume.js`

```
player.js  ──import from player/index.js──→  player/index.js
                                                  │
                                          re-exports from player/resume.js
                                                  │
                                                  ▼
                                           player/resume.js
                                                  │
                                          import { playItem } from '../player.js'
                                                  │
                                                  ▼
                                           player.js  ← 循环！
```

**风险**：`player/resume.js` 导入 `playItem`（来自父级 `player.js`），而 `player.js` 又从 `player/index.js` 导入 `player/resume.js` 的导出。

**建议**：将 `playItem` 移入 `player/` 子目录（如 `player/core.js` 或新建 `player/play.js`），打破循环。

---

## 3. 死代码 / 未使用的导出

### 3.1 `I18N`（i18n.js）

`i18n.js` 导出了 `I18N` 常量（翻译数据对象），但没有任何模块导入它。所有翻译通过 `t(key)` 函数访问。

**建议**：移除 `export` 关键字，改为模块内部变量。

### 3.2 `mediaErrorText`（api.js）

`api.js` L84 导出了 `mediaErrorText` 函数，但没有任何模块导入它。

**建议**：移除导出或删除函数。

### 3.3 `switchAudioTrack` / `getAudioTracks`（player.js）

通过 `player/index.js` → `player.js` 导出链暴露，但没有任何外部模块导入这两个函数。它们只在 `player/core.js` 内部通过 `applyPlyr` 间接使用。

**建议**：从 `player/index.js` 和 `player.js` 的导出列表中移除。

### 3.4 `getSortVal`（playlist/sort-filter.js）

导出但未被任何外部模块导入。只在 `sort-filter.js` 内部被 `sortFiles` 使用。

**建议**：移除导出。

### 3.5 `hidePinDialog` / `verifyPin`（pin.js）

导出但只在 `pin.js` 内部使用。

**建议**：移除导出。

### 3.6 `canStorage` 重复导出

`player/core.js` 导出 `canStorage`，通过 `player/index.js` → `player.js` 链暴露。但外部模块使用的是 `state.js` 中的版本。`player/core.js` 内部也使用自己的版本。

**建议**：`player/core.js` 改为从 `state.js` 导入 `canStorage`，移除自己的定义和导出。

---

## 4. 无监听者的 bus 事件

### 4.1 `bus.emit('playlist:updated')`

在 `playlist/navigation.js` L51 发射，但整个代码库中没有任何 `bus.on('playlist:updated', ...)` 监听器。

**建议**：如果不需要，移除该 `bus.emit` 调用。如果计划使用，添加相应的监听器。

### 4.2 `bus.emit('ui:render')`

在 `ui/render.js` L54 的 `updateUIForLang()` 中发射，但整个代码库中没有任何 `bus.on('ui:render', ...)` 监听器。

**建议**：同上。

---

## 5. 架构模式不一致

### 5.1 `player.js` 和 `playlist.js` 的"超级包装器"模式

`ui.js` 被正确重构为薄编排层：
- 从 `ui/` 子模块导入 → re-export → 注册 bus 监听器
- 不包含自己的业务逻辑

但 `player.js` 和 `playlist.js` 没有遵循同样的模式：
- 它们从子目录导入 **部分** 函数，但也保留了自己的重复实现
- `player.js` 有 480 行，包含 `playItem()`、`resumeLast()`、`bindGlobalHotkeys()` 等大型函数
- `playlist.js` 有 175 行，包含自己的 `renderPlaylist()` 和 `playAtIndex()`

**建议**：将 `player.js` 和 `playlist.js` 也重构为薄编排层（类似 `ui.js`），把所有业务逻辑下沉到子目录模块。

### 5.2 混合导入路径

有些模块从父级导入，有些从子目录直接导入：

| 模块 | 从父级导入 | 从子目录直接导入 |
|------|-----------|-----------------|
| `ui/bindings.js` | `../playlist.js`、`../player.js` | — |
| `ui/settings.js` | `../playlist.js`、`../player.js` | — |
| `player.js` | — | `./player/index.js`（barrel） |
| `player/resume.js` | `../player.js`、`../playlist.js` | `./seek.js`（直接） |
| `ui.js` | `./player.js` | `./ui/bindings.js`、`./ui/render.js` 等（绕过 barrel） |

**建议**：统一规则——要么都通过 barrel 导入，要么都直接导入。推荐通过 barrel 导入以保持封装性。

### 5.3 `ui/bindings.js` 反向依赖顶层 `actions.js`

`ui/bindings.js`（UI 子模块）导入了 `actions.js`（顶层编排器）的 `loadConfig` 和 `loadMedia`。这违反了分层原则——底层模块不应该依赖顶层模块。

**建议**：通过 bus 事件解耦（见 2.1 节建议）。

---

## 6. 修复优先级建议

### P0（应该尽快修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| 1.1 | `renderPlaylist()` 重复 | 两份实现可能行为不一致 |
| 1.3 | `resumeLast()` 重复 | 修改一处不会同步 |
| 1.4 | `bindGlobalHotkeys()` 重复 | 修改一处不会同步 |

### P1（下一迭代修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| 1.2 | `playAtIndex()` 不一致 | 播放列表刷新行为可能不同 |
| 1.5 | `rememberEnabled()` 三份 | 代码冗余 |
| 2.1 | `actions ↔ ui/bindings` 循环 | 架构脆弱 |
| 2.2 | `player.js ↔ player/resume.js` 循环 | 架构脆弱 |

### P2（有空时清理）

| 编号 | 问题 | 影响 |
|------|------|------|
| 1.6 | `canStorage()` 重复 | 代码冗余 |
| 3.x | 死代码导出 | 增加认知负担 |
| 4.x | 无监听者的 bus 事件 | 误导性代码 |
| 5.x | 架构模式不一致 | 降低可维护性 |

---

## 7. 理想的目标架构

```
app.js
  └── modules/actions.js        ← 薄编排层：boot() + loadConfig() + loadMedia()
        ├── modules/ui.js        ← 薄编排层：re-export + bus 监听（✅ 已完成）
        │     └── ui/            ← bindings, render, shares, settings
        ├── modules/player.js    ← 薄编排层：re-export + bus 监听（🔴 待重构）
        │     └── player/        ← core, seek, resume, transcode, audio-track
        ├── modules/playlist.js  ← 薄编排层：re-export + bus 监听（🔴 待重构）
        │     └── playlist/      ← navigation, render, sort-filter
        ├── modules/theme.js     ← 独立模块
        ├── modules/pin.js       ← 独立模块
        ├── modules/i18n.js      ← 独立模块
        ├── modules/api.js       ← 基础设施
        ├── modules/state.js     ← 基础设施
        ├── modules/eventbus.js  ← 基础设施
        ├── modules/utils.js     ← 工具函数
        ├── modules/icons.js     ← SVG 图标
        └── modules/lyrics.js    ← 歌词处理
```

每个父级模块（`player.js`、`playlist.js`）应像 `ui.js` 一样：
1. 从子目录 barrel 导入 → re-export 公共接口
2. 注册 bus 监听器（如果需要）
3. **不包含自己的业务逻辑实现**
