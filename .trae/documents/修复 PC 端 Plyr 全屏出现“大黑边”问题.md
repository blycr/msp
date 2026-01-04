## 根因分析
- 选择器不匹配：项目自定义样式使用了 .plyr--fullscreen-active，但 Plyr 实际使用的是原生伪类 .plyr:fullscreen 与回退类 .plyr--fullscreen-fallback（参见 [plyr.css](file:///c:/Users/blycr/msp/web/assets/plyr/plyr.css#L1145-L1204)）。
- 高度被 ID 规则限制：非全屏下为布局添加的 `#videoEl{ max-height:72vh }`（见 [app.css](file:///c:/Users/blycr/msp/web/app.css#L222-L226)）在“容器进入原生全屏”时仍生效，因为 `:fullscreen` 作用在容器而非 video 元素；导致视频在全屏容器中仍被 72vh 限制，从而出现四周大黑边。
- 固定宽高比未关闭：Plyr 的包裹器使用固定宽高比（`padding-bottom` 或 `aspect-ratio`，见 [plyr.css](file:///c:/Users/blycr/msp/web/assets/plyr/plyr.css#L985-L1016)）。若全屏时不显式取消，会造成内容未铺满容器。
- JS 层情况：仓库中未实现任何全屏事件监听或样式切换（见 [app.js](file:///c:/Users/blycr/msp/web/app.js#L576-L622)）。未发现内联样式动态覆盖导致的冲突，问题主要来自 CSS 选择器与优先级。

## 解决方案要点
1. 用正确的选择器覆盖：改用 `.plyr:fullscreen` 与 `.plyr--fullscreen-fallback`，不要使用 `.plyr--fullscreen-active`。
2. 在全屏时解除 `#videoEl` 的高度上限，并让视频与包裹器充满可用空间。
3. 关闭全屏下的固定宽高比（取消 `padding-bottom` / 强制 100% 宽高）。
4. 视需求切换 `object-fit`：
   - 保持比例且可能留边：`contain`
   - 铺满屏幕可能裁剪：`cover`
5. 可选：添加全屏事件监听以做更细致控制（如切换 UI、滚动锁定等）。

## 拟修改内容（CSS 片段）
- 合并到 `web/app.css`，并置于最后，确保覆盖优先级足够。

```css
/* 非全屏保留原限制 */
#videoEl { max-height: 72vh; }

/* 容器进入原生全屏或回退全屏时，解除视频限制 */
.plyr:fullscreen #videoEl,
.plyr--fullscreen-fallback #videoEl {
  max-height: none !important;
  width: 100%;
  height: 100%;
  object-fit: contain; /* 或 cover，按业务需选择 */
}

/* 全屏时取消固定宽高比并让包裹器铺满 */
.plyr:fullscreen .plyr__video-wrapper,
.plyr--fullscreen-fallback .plyr__video-wrapper {
  padding-bottom: 0 !important;
  height: 100% !important;
  width: 100% !important;
}

/* 如需进一步确保容器充满视口（回退模式） */
.plyr--fullscreen-fallback {
  inset: 0 !important;
  position: fixed !important;
  z-index: 2147483647 !important;
  background: #000;
}
```

## 可选增强（JS）
- 监听原生全屏切换以进行更细致控制（例如暂时禁止滚动）。

```js
// 放在 Plyr 初始化后
document.addEventListener('fullscreenchange', () => {
  const isFull = !!document.fullscreenElement;
  document.documentElement.style.overflow = isFull ? 'hidden' : '';
});
```
- 如需监听 Plyr 事件（版本支持时）：

```js
// 示例：根据库版本决定事件名是否可用
// state.plyr.on('enterfullscreen', () => { /* ... */ });
// state.plyr.on('exitfullscreen', () => { /* ... */ });
```

## 验证与兼容性
- 在 Chrome 与 Edge：
  - 打开 DevTools，点击全屏后确认 `.plyr:fullscreen` 或 `.plyr--fullscreen-fallback` 是否出现。
  - 检查 `#videoEl` 的计算样式：`max-height` 应为 `none`，`height/width` 为 `100%`。
  - 观察黑边是否仅保留为正常的“信箱式”留边（`contain` 情况），或在 `cover` 下是否完全铺满。
- 参考文件：
  - CSS 覆盖处：[app.css](file:///c:/Users/blycr/msp/web/app.css#L227-L273)
  - Plyr 全屏样式（官方）：[plyr.css](file:///c:/Users/blycr/msp/web/assets/plyr/plyr.css#L1145-L1204)
  - 初始化位置： [app.js](file:///c:/Users/blycr/msp/web/app.js#L576-L622)

## 交付与风险
- 变更仅限 CSS，无副作用；若选择 `cover` 可能出现画面裁剪，这是预期行为。
- 若页面存在对 `position: fixed` 的异常影响（例如奇特的浏览器 bug 或复杂的滚动容器），保留 JS 监听作为最后保障。

请确认以上方案；确认后我将按该方案更新 CSS 并进行本地验证（Chrome/Edge），确保问题消失并给出截图/录屏与差异说明。