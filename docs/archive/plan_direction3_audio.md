# 🚫 方向三：音频体验深化 — 实施计划 【已搁置 / ON HOLD】

> ⚠️ **本计划全文已搁置，请勿执行。**
>
> 原因：未经充分验证。存在 DOM 元素假设错误（`playlistBox` 不存在）、`queueActive` 状态未闭环、队列与正常播放冲突未处理、音频可视化方案可能干扰歌词滚动等问题。需重新设计后再决定是否重启。
>
> **兜底原则**：同方向二。

---

## 特性 3A：播放队列（跨 Share 自由组合）

**场景**：当前 playlist 固定为某 scope 下全部音频。用户想从不同目录挑选歌曲组成队列。
**复杂度**：Light — 纯前端改动。

### 3A.1 状态扩展

**文件**：`web/src/modules/state.js`

在 `state` 对象添加：
```js
queue: [],           // Array<MediaItem> — 用户手动添加的播放队列
queueActive: false,  // 队列模式是否激活
```

### 3A.2 队列操作 API

**文件**：新建 `web/src/modules/queue.js`

```js
import { state } from './state.js';
import { bus } from './eventbus.js';
import { setPlaylist, generatePlayOrder } from './playlist.js';

export function addToQueue(item) {
  // 去重
  if (state.queue.some(q => q.id === item.id)) return;
  state.queue.push(item);
  bus.emit('queue:updated');
}

export function removeFromQueue(index) {
  state.queue.splice(index, 1);
  bus.emit('queue:updated');
}

export function clearQueue() {
  state.queue = [];
  state.queueActive = false;
  bus.emit('queue:updated');
}

export function playQueue() {
  if (!state.queue.length) return;
  state.queueActive = true;
  const items = [...state.queue];
  const playOrder = generatePlayOrder(items.length, 0, state.playlist.shuffle);
  setPlaylist('audio', items, 0, playOrder, 0);
  bus.emit('play:request', items[0], { fromPlaylist: true, autoplay: true, user: true });
}

export function moveInQueue(fromIdx, toIdx) {
  if (fromIdx < 0 || toIdx < 0 || fromIdx >= state.queue.length || toIdx >= state.queue.length) return;
  const [item] = state.queue.splice(fromIdx, 1);
  state.queue.splice(toIdx, 0, item);
  bus.emit('queue:updated');
}
```

### 3A.3 列表项"添加到队列"按钮

**文件**：`web/src/modules/ui/render.js`

在 renderList 中，对 `kind === 'audio'` 的项目，在 badge 之后 / favBtn 之前添加：

```js
if (item.kind === 'audio') {
  const queueBtn = document.createElement('button');
  queueBtn.className = 'queue-btn';
  queueBtn.textContent = '+';
  queueBtn.title = t('add_to_queue');
  queueBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    addToQueue(item);
  });
  row.appendChild(queueBtn);
}
```

import `addToQueue` from `'../queue.js'`。

### 3A.4 队列面板 UI

**文件**：`web/src/modules/ui/render.js`（或新建 `web/src/modules/ui/queue.js`）

```js
import { bus } from '../eventbus.js';
import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { formatName } from '../utils.js';
import { playQueue, removeFromQueue, clearQueue } from '../queue.js';

bus.on('queue:updated', renderQueuePanel);

function renderQueuePanel() {
  let panel = document.getElementById('queuePanel');
  if (!panel) {
    panel = document.createElement('div');
    panel.id = 'queuePanel';
    panel.className = 'queue-panel';
    // 插入到 playlist 容器附近
    const playlistBox = el('playlistBox');
    if (playlistBox) playlistBox.parentNode.insertBefore(panel, playlistBox.nextSibling);
  }

  if (!state.queue.length) {
    panel.hidden = true;
    return;
  }
  panel.hidden = false;

  panel.innerHTML = `
    <div class="queue-panel__header">
      <span class="queue-panel__title">${t('queue')} (${state.queue.length})</span>
      <button class="btn btn--ghost btn--sm" id="queuePlayBtn">${t('queue_play')}</button>
      <button class="btn btn--ghost btn--sm" id="queueClearBtn">${t('queue_clear')}</button>
    </div>
    <div class="queue-panel__list"></div>
  `;

  const listEl = panel.querySelector('.queue-panel__list');
  state.queue.forEach((item, i) => {
    const row = document.createElement('div');
    row.className = 'queue-panel__item';
    row.innerHTML = `
      <span class="queue-panel__name">${formatName(item)}</span>
      <button class="queue-panel__remove" data-idx="${i}">×</button>
    `;
    listEl.appendChild(row);
  });

  panel.querySelector('#queuePlayBtn').addEventListener('click', playQueue);
  panel.querySelector('#queueClearBtn').addEventListener('click', clearQueue);
  panel.querySelectorAll('.queue-panel__remove').forEach(btn => {
    btn.addEventListener('click', (e) => {
      removeFromQueue(Number(btn.dataset.idx));
    });
  });
}
```

> **兜底**：确保此文件在 `web/src/modules/ui/index.js` 或入口文件中被 import，否则 bus 监听不会注册。

### 3A.5 样式

**文件**：`web/src/styles/components/list.css`（追加）

```css
/* Queue */
.queue-btn {
  background: none; border: 1px solid var(--md-border);
  border-radius: 50%; width: 24px; height: 24px;
  cursor: pointer; font-size: 14px; line-height: 1;
  color: var(--md-on-surface-variant); flex-shrink: 0;
  transition: all 0.15s;
}
.queue-btn:hover { background: var(--md-primary); color: #fff; border-color: var(--md-primary); }
.queue-panel { padding: 8px 0; }
.queue-panel[hidden] { display: none; }
.queue-panel__header {
  display: flex; align-items: center; gap: 8px;
  padding: 4px 12px; font-size: 12px;
}
.queue-panel__title { flex: 1; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; color: var(--md-on-surface-variant); }
.queue-panel__item {
  display: flex; align-items: center; padding: 4px 12px; font-size: 13px;
}
.queue-panel__name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.queue-panel__remove {
  background: none; border: none; cursor: pointer;
  color: var(--md-on-surface-variant); font-size: 16px; padding: 2px 6px;
}
.queue-panel__remove:hover { color: var(--md-error, #e53935); }
```

### 3A.6 i18n

en:
```js
add_to_queue: "Add to queue",
queue: "Queue",
queue_play: "Play all",
queue_clear: "Clear",
```

zh:
```js
add_to_queue: "加入队列",
queue: "播放队列",
queue_play: "播放全部",
queue_clear: "清空",
```

### 3A.7 验证清单

- [ ] `cd web && bun run build` 通过
- [ ] 音频列表项显示 "+" 按钮
- [ ] 点击 "+" 将音频添加到队列面板
- [ ] 队列面板显示在播放列表下方
- [ ] 点击"播放全部"替换当前 playlist 并开始播放
- [ ] 可以从队列移除单项
- [ ] 可以清空队列

---

## 特性 3B：音频可视化

**场景**：纯前端改动，提升音乐播放沉浸感。
**复杂度**：Light — `<canvas>` + Web Audio API。

### 3B.1 可视化模块

**文件**：新建 `web/src/modules/player/visualizer.js`

```js
import { state, el } from '../state.js';
import { bus } from '../eventbus.js';

let audioCtx = null;
let analyser = null;
let source = null;
let animFrameId = 0;
let canvas = null;
let ctx = null;
let connectedElement = null;

export function initVisualizer() {
  canvas = document.getElementById('audioVisualizer');
  if (!canvas) return;
  ctx = canvas.getContext('2d');
}

function connectToAudio(mediaEl) {
  if (!canvas || !mediaEl) return;
  // 防止重复连接同一元素
  if (connectedElement === mediaEl) return;

  try {
    if (!audioCtx) audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    if (source) { /* 不能 disconnect MediaElementSource，只能复用 */ }
    if (!source) {
      source = audioCtx.createMediaElementSource(mediaEl);
    }
    analyser = audioCtx.createAnalyser();
    analyser.fftSize = 256;
    source.connect(analyser);
    analyser.connect(audioCtx.destination);
    connectedElement = mediaEl;
    draw();
  } catch (e) {
    console.warn('Visualizer init failed:', e);
  }
}

function draw() {
  if (!analyser || !ctx || !canvas) return;
  animFrameId = requestAnimationFrame(draw);

  const bufLen = analyser.frequencyBinCount;
  const data = new Uint8Array(bufLen);
  analyser.getByteFrequencyData(data);

  const w = canvas.width = canvas.offsetWidth * (window.devicePixelRatio || 1);
  const h = canvas.height = canvas.offsetHeight * (window.devicePixelRatio || 1);
  ctx.clearRect(0, 0, w, h);

  const barCount = Math.min(bufLen, 64);
  const barWidth = w / barCount;
  const style = getComputedStyle(document.documentElement);
  const primary = style.getPropertyValue('--md-primary').trim() || '#6750A4';

  for (let i = 0; i < barCount; i++) {
    const barHeight = (data[i] / 255) * h * 0.8;
    const x = i * barWidth;
    ctx.fillStyle = primary;
    ctx.globalAlpha = 0.6 + (data[i] / 255) * 0.4;
    ctx.fillRect(x, h - barHeight, barWidth - 1, barHeight);
  }
  ctx.globalAlpha = 1;
}

export function stopVisualizer() {
  cancelAnimationFrame(animFrameId);
  animFrameId = 0;
  if (ctx && canvas) ctx.clearRect(0, 0, canvas.width, canvas.height);
}

// 监听播放事件
bus.on('play:started', ({ kind, mediaEl }) => {
  if (kind === 'audio' && mediaEl) {
    initVisualizer();
    connectToAudio(mediaEl);
  } else {
    stopVisualizer();
  }
});

bus.on('play:stopped', stopVisualizer);
```

> **兜底**：
> 1. `bus.on('play:started', ...)` 需要 play.js 在播放成功后 emit 此事件并传递 mediaEl。检查 play.js 是否已有类似事件，若无则添加 `bus.emit('play:started', { kind: item.kind, mediaEl })` 到 playItem 成功路径。
> 2. Web Audio API 的 `createMediaElementSource` 只能对同一 element 调用一次。如果用户切歌，需要复用同一 source。上面的代码已处理此情况。
> 3. 某些浏览器在用户交互前不允许创建 AudioContext（autoplay policy）。`connectToAudio` 应在用户点击播放后调用，已通过 bus 事件确保。

### 3B.2 HTML：Canvas 元素

**文件**：`web/index.html`

在音频播放器区域（`audioMeta` 或 `playerBox` 内），音频封面/歌词下方添加：
```html
<canvas id="audioVisualizer" class="audio-visualizer"></canvas>
```

> **兜底**：确切位置取决于 index.html 的音频播放器 DOM 结构。放在 `#playerBox` 内部、音频控件下方即可。

### 3B.3 样式

**文件**：`web/src/styles/components/player.css`（追加）

```css
/* Audio Visualizer */
.audio-visualizer {
  width: 100%;
  height: 48px;
  display: block;
  margin-top: 4px;
  border-radius: 4px;
  opacity: 0.8;
}
```

### 3B.4 入口导入

**文件**：确保 `visualizer.js` 被导入。在 `web/src/modules/player/index.js`（或 `player.js`）中添加：
```js
import './visualizer.js';
```

### 3B.5 play.js：emit play:started 事件

**文件**：`web/src/modules/player/play.js`

在 `playItem` 函数中，媒体成功开始播放后（`canplay` 或播放器 ready 回调中），添加：
```js
bus.emit('play:started', { kind: item.kind, mediaEl: mediaElement });
```

其中 `mediaElement` 是 `document.getElementById('audioEl')` 或 `document.getElementById('videoEl')`。

在 `destroyPlyr` 或媒体切换清理时，添加：
```js
bus.emit('play:stopped');
```

> **兜底**：如果 play.js 中获取 mediaElement 的方式不同，按实际代码适配。

### 3B.6 验证清单

- [ ] `cd web && bun run build` 通过
- [ ] 播放音频时，播放器下方显示频谱动画
- [ ] 切换到视频时，频谱自动停止
- [ ] 暂停时频谱停止动画
- [ ] 频谱颜色跟随主题 `--md-primary`
