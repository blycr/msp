## 原因分析
- 选择器不匹配：Plyr 使用 .plyr:fullscreen 与 .plyr--fullscreen-fallback；已在本地样式采用正确选择器，但需确认实际进入全屏的目标元素是容器还是 video 元素。
- 高优先级规则：#videoEl 的 max-height:72vh 在非全屏有效；需确保在全屏上下文解除该限制并使 video/wrapper 撑满。
- 宽高比与填充策略：object-fit: contain 会在横屏显示竖屏视频时产生黑边；若追求“无黑边”，需在全屏时切换为 cover。
- Plyr 版本与配置：未显式启用 fullscreen.fallback 时，某些环境下可能仅启用原生全屏或按钮策略不同，导致样式未全部生效。
- 事件与行为：未绑定 enterfullscreen/exitfullscreen 钩子，无法在进入/退出全屏时动态修正样式和记录真实全屏节点，排查难度较大。

## 修复步骤
1. 显式启用 Plyr 全屏配置
- 在 applyPlyr 中设置 opts.fullscreen = { enabled: true, fallback: true }
- 代码位置参考：[app.js](file:///c:/Users/blycr/msp/web/app.js#L604-L616)

2. 绑定 Plyr 全屏事件并动态调整填充策略
- 进入全屏时将 video 的 object-fit 切为 cover；退出全屏恢复 contain
- 同时记录实际全屏元素，辅助定位问题

```js
state.plyr.on("enterfullscreen", () => {
  try { element.style.objectFit = "cover"; } catch {}
  try { console.log(document.fullscreenElement); } catch {}
});
state.plyr.on("exitfullscreen", () => {
  try { element.style.objectFit = "contain"; } catch {}
});
```

3. 补充 CSS，确保全屏下完全铺满
- 为全屏下的 video 强化填充策略（默认 cover，可按需改回 contain）
- 代码位置参考：[app.css](file:///c:/Users/blycr/msp/web/app.css#L262-L281)

```css
.plyr:fullscreen video,
.plyr--fullscreen-fallback video{
  width:100% !important;
  height:100% !important;
  object-fit:cover !important;
}
#videoEl:fullscreen{width:100%; height:100%; object-fit:cover}
#videoEl:-webkit-full-screen{width:100%; height:100%; object-fit:cover}
```

4. 维持 wrapper 在全屏时去掉固有宽高比
- 保持 .plyr__video-wrapper 在全屏上下文 height/width:100%，去除 padding-bottom
- 位置参考：[app.css](file:///c:/Users/blycr/msp/web/app.css#L248-L261)

5. 加入调试输出与安全兜底
- 在 document fullscreenchange 时输出 document.fullscreenElement 的节点类型
- 若实际进入全屏的是 video 而非 .plyr 容器，确认选择器覆盖到 #videoEl:fullscreen
- 位置参考：[app.js](file:///c:/Users/blycr/msp/web/app.js#L36-L39)

```js
document.addEventListener("fullscreenchange", () => {
  const el = document.fullscreenElement;
  try { console.log(el && (el.id || el.className || el.tagName)); } catch {}
});
```

6. 可选：增加“填充模式”切换
- 在 UI 增加一个按钮在 contain 与 cover 间切换，满足不同视频宽高比场景
- 进入全屏默认 cover，退出全屏恢复 contain

## 验证清单
- 在 Chrome/Edge 桌面环境分别验证 16:9、4:3、9:16 视频的全屏效果
- 在存在/不存在字幕轨道时验证控件遮罩与铺满行为
- 验证回退全屏（.plyr--fullscreen-fallback）是否同样铺满
- 记录 fullscreenchange 输出，确认实际全屏元素与选择器匹配

## 代码参考
- 样式入口：[app.css](file:///c:/Users/blycr/msp/web/app.css)
- Plyr 样式参考：[plyr.css](file:///c:/Users/blycr/msp/web/assets/plyr/plyr.css#L1145-L1206)
- 逻辑入口与初始化：[app.js](file:///c:/Users/blycr/msp/web/app.js#L581-L627)
- 页面结构：[index.html](file:///c:/Users/blycr/msp/web/index.html#L57-L66)

请确认以上方案；确认后我将按步骤修改并进行本地验证。