# MSP 前端样式审计报告

> 生成时间：2026-05-14
> 审计工具：Puppeteer Coverage API + 人工代码审查
> 目标文件：`web/src/app.css`（1,385 行）

---

## 1. 执行摘要

| 指标 | 数值 | 评级 |
|------|------|------|
| 样式文件数 | 1（全部集中） | ⚠️ 需拆分 |
| 总行数 | 1,385 | ⚠️ 偏长 |
| `!important` 数量 | 22 | 🔴 过多 |
| ID 选择器数量 | 13 | ⚠️ 偏高 |
| HTML 内联 `style` | 11 处 | 🔴 应消除 |
| JS 硬编码 `el.style.*` | 11+ 处 | 🔴 应消除 |
| 重复定义（同一选择器多处定义） | 5 组 | 🔴 严重 |
| Coverage 使用率 | 68.6% | ⚠️ 有优化空间 |

**核心发现：**
- `app.css` 中存在 **5 组重复定义**，其中前一组（第 150-183 行）是带 `!important` 的"补丁式"定义，后一组（第 389-1028 行）是完整的组件定义
- 补丁组的 `!important` 导致组件定义中的 `color` 等属性被强制覆盖，造成了**设计意图冲突**
- 31.4% 的"未使用 CSS"大部分是正常的（hover/focus/响应式/主题切换态未触发），但 `h1-h6` 和 `textarea` 样式确认是**死代码**

---

## 2. 重复定义详细分析（🔴 最高优先级）

### 2.1 `.btn` — 第 150 行 vs 第 978 行

**补丁组（第 150-157 行）：**
```css
.btn {
  background: var(--md-primary) !important;
  color: #fff !important;
}
.btn:hover {
  background: var(--md-primary-2) !important;
}
```

**完整组（第 978-1006 行）：**
```css
.btn {
  border: 1px solid rgba(0, 0, 0, 0.05);
  background: var(--md-primary);
  color: #fff;
  font-weight: 600;
  height: 36px;
  padding: 0 16px;
  /* ... 共 13 条属性 */
}
.btn:hover {
  background: var(--md-primary-2);
  transform: translateY(-1px);
  box-shadow: 0 10px 15px -3px rgba(0,0,0,0.15), ...;
}
.btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 4px -1px rgba(0,0,0,0.1);
}
```

**冲突分析：**
- 补丁组只有 `background` + `color`（`!important`），完整组有 13 条属性
- 两段定义的 `background`/`color` 值**完全相同**，所以视觉上无差异
- 补丁组没有 `.btn:active` 定义，active 状态完全依赖完整组

**结论：第 150-157 行是冗余补丁，可安全删除。**

---

### 2.2 `.btn--ghost` — 第 159 行 vs 第 1008 行

**补丁组（第 159-171 行）：**
```css
.btn--ghost {
  background: transparent !important;
  color: var(--md-primary) !important;          /* ← Indigo 色 */
  border: 1px solid var(--md-border) !important;
}
.btn--ghost:hover {
  background: var(--md-hover) !important;
}
[data-theme="dark"] .btn--ghost {
  color: var(--md-text) !important;              /* ← 白色 */
}
```

**完整组（第 1008-1028 行）：**
```css
.btn--ghost {
  background: transparent;
  color: var(--md-sub);                          /* ← Zinc 灰 */
  border: 1px solid var(--md-border);
  box-shadow: none;
}
.btn--ghost:hover {
  background: var(--md-hover);
  color: var(--md-text);
  border-color: var(--md-sub);
  transform: translateY(-1px);
}
[data-theme="dark"] .btn--ghost {
  color: var(--md-sub);                          /* ← Zinc 灰 */
}
[data-theme="dark"] .btn--ghost:hover {
  color: var(--md-text);                         /* ← 白色 */
}
```

**冲突分析：**
- **Light 主题默认状态：补丁组胜出**（`!important` 锁定为 `var(--md-primary)` / Indigo）
- **Light 主题 hover：完整组胜出**（补丁组未定义 hover color，所以 `var(--md-text)` 生效）
- **Dark 主题默认状态：补丁组胜出**（`!important` 锁定为 `var(--md-text)` / 白色）
- **Dark 主题 hover：完整组胜出**（`var(--md-text)`，和补丁组一致）

**实际效果 vs 完整组意图：**

| 状态 | 当前实际颜色 | 完整组意图颜色 | 差异 |
|------|-------------|---------------|------|
| Light 默认 | Indigo (`--md-primary`) | Zinc Gray (`--md-sub`) | 🔴 ghost 按钮比设计意图更突出 |
| Light hover | Zinc 900 (`--md-text`) | Zinc 900 (`--md-text`) | ✅ 一致 |
| Dark 默认 | White (`--md-text`) | Zinc 400 (`--md-sub`) | 🔴 ghost 按钮比设计意图更突出 |
| Dark hover | White (`--md-text`) | White (`--md-text`) | ✅ 一致 |

**结论：这是一个设计决策冲突。** 补丁组让 ghost 按钮默认状态下使用主色/白色，更醒目；完整组意图让它使用次要色，更低调。**需要产品侧确认偏好。**

---

### 2.3 `.tab` — 第 177 行 vs 第 389 行

**补丁组（第 177-179 行）：**
```css
.tab {
  color: var(--md-text) !important;    /* ← Zinc 900 / White */
}
```

**完整组（第 389-401 行）：**
```css
.tab {
  flex: 1;
  border: 1px solid transparent;
  background: transparent;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-weight: 600;
  color: var(--md-sub);                /* ← Zinc 500 / 400 */
  font-size: 14px;
  transition: all 0.2s ease;
  text-align: center;
}
```

**冲突分析：**
- 补丁组强制 tab 默认颜色为文字主色，完整组意图为次要色
- `.tab:hover`（第 403 行）定义 `color: var(--md-text)` — 如果默认已经是 `var(--md-text)`，hover 无颜色变化，视觉反馈减弱
- `.tab--active`（第 408 行）定义 `color: var(--md-primary)` — active tab 用主色区分

**当前效果：** 未选中 tab 和选中 tab 在颜色上区分度不足（都是接近文字主色，只差一个 font-weight）。

**结论：第 177 行是冗余补丁。删除后，未选中 tab 用次要色，选中 tab 用主色，层次更清晰。**

---

### 2.4 `.tab--active` — 第 181 行 vs 第 408 行

**补丁组（第 181-184 行）：**
```css
.tab--active {
  color: var(--md-text) !important;      /* ← 已被覆盖 */
  background: var(--md-active) !important;
}
```

**完整组（第 408-412 行）：**
```css
.tab--active {
  background: var(--md-active) !important;
  color: var(--md-primary) !important;   /* ← 后定义，实际生效 */
  font-weight: 700;
}
```

**冲突分析：**
- 两段都有 `!important`，后定义（第 408 行）覆盖先定义（第 181 行）
- `background` 值相同，无冲突
- `color` 值不同：第 181 行 `var(--md-text)`，第 408 行 `var(--md-primary)`
- **实际生效的是第 408 行的 `var(--md-primary)`**

**结论：第 181 行是 100% 死代码。** `color` 被覆盖，`background` 冗余。

---

### 2.5 `.theme-btn` — 第 173 行 vs 第 554 行

**补丁组（第 173-175 行）：**
```css
.theme-btn {
  color: var(--md-text) !important;    /* ← Zinc 900 / White */
}
```

**完整组（第 554-567 行）：**
```css
.theme-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 8px;
  border-radius: var(--radius-sm);
  transition: background-color 0.2s ease, transform 0.15s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  color: var(--md-sub);                /* ← Zinc 500 / 400 */
}
```

**冲突分析：**
- 和 `.tab` 的情况类似：补丁组强制主文字色，完整组意图次要色
- `.theme-btn:hover`（第 577 行）定义 `color: var(--md-text)` — 如果默认已经是主色，hover 无变化

**结论：第 173 行是冗余补丁。删除后，theme-btn 默认次要色、hover 主色，有明确的交互反馈。**

---

## 3. `!important` 审计

| 位置 | 规则 | 必要性 | 建议 |
|------|------|--------|------|
| 83 | `[hidden] { display: none !important; }` | ✅ 合理 | 保留，工具类需要最高优先级 |
| 150-183 | 补丁组（12 个） | 🔴 疑似过时 | 删除补丁组后，这 12 个自然消失 |
| 409-410 | `.tab--active` | ⚠️ 有争议 | 可用更高特异性替代（如 `.tabs .tab--active`） |
| 1364-1384 | Plyr 覆盖（5 个） | ✅ 合理 | 第三方库覆盖需要 `!important` |

**如果删除补丁组：`!important` 从 22 个降至 10 个，减少 55%。**

---

## 4. 死代码确认（Coverage + 人工审查）

| 规则 | 行号 | 死代码原因 | 安全删除 |
|------|------|-----------|---------|
| `h1, h2, h3, h4, h5, h6` | 213-224 | `index.html` 中没有任何 h1-h6 标签 | ✅ |
| `textarea:focus-visible` | 143-147 | 页面中没有 `<textarea>` 元素 | ✅（但 `select:focus-visible` 保留，select 会被重构） |
| `.tab--active` 补丁组 | 181-184 | `color` 被第 408 行覆盖，`background` 冗余 | ✅ |

**注意：** Coverage 报告中的 31.4% "未使用"大部分是**条件性未使用**（hover/focus/响应式/主题态未触发），不是死代码。只有上表中的 3 处是确认的死代码。

---

## 5. 其他维护性问题

### 5.1 单一文件爆炸
全部 1,385 行在一个文件中，按组件拆分可显著提升可维护性：

```
src/styles/
├── base.css          (变量、字体、reset、全局过渡)
├── layout.css        (layout, topbar, sidebar, stage, panel)
├── components/
│   ├── button.css    (.btn, .btn--ghost)
│   ├── form.css      (.textfield, .toggle, .search, select → dropdown)
│   ├── list.css      (.item, .list__body, .pager, .badge, .hint)
│   ├── dialog.css    (.dialog, .dialog__*, .share)
│   ├── player.css    (.playerbox, .now, .audioMeta, .lyrics)
│   └── playlist.css  (.playlist, .plitem, .tabs, .tab)
└── vendor/
    └── plyr.css      (Plyr 覆盖)
```

### 5.2 命名不统一
- BEM 风格：`.topbar__title`, `.item__name` ✅
- 单词风格：`.badge`, `.hint`, `.pager`
- 建议：统一为 BEM（如 `.list__badge`, `.list__hint`）

### 5.3 内联样式分散
- **HTML**：11 处 `style="..."`，主要是 `flex: 1` 布局调整
- **JS**：多处 `el.style.display = "none"` / `"block"`
- 建议：提取为语义化 class（如 `.flex-grow`, `.hidden`）

### 5.4 ID 选择器混用
13 个 ID 选择器（特异性 100）和类选择器混用。虽然当前没有覆盖需求，但增加了未来扩展的难度。

---

## 6. 风险评估

| 改动 | 风险 | 缓解措施 |
|------|------|---------|
| 删除 `.btn` 补丁组 | 🟢 低 | 值相同，只有 `!important` 差异 |
| 删除 `.tab` 补丁组 | 🟢 低 | 视觉改善（层次更清晰） |
| 删除 `.tab--active` 补丁组 | 🟢 低 | 100% 死代码 |
| 删除 `.theme-btn` 补丁组 | 🟢 低 | 视觉改善（hover 反馈恢复） |
| 删除 `.btn--ghost` 补丁组 | 🟡 中 | **颜色从 Indigo 变为 Zinc Gray**，需确认设计意图 |
| 删除 `h1-h6` / `textarea` | 🟢 低 | 确认无使用 |
| 文件拆分 | 🟡 中 | 纯文件移动，不改选择器和属性值 |
| 自定义 dropdown 替换 select | 🟡 中 | 需要 JS 实现，需测试键盘导航 |

---

## 7. 修复建议（按优先级排序）

### P0：安全清理（零风险）
1. 删除第 181 行 `.tab--active` 补丁组（100% 死代码）
2. 删除第 213-224 行 `h1-h6` 样式
3. 删除 `textarea:focus-visible`（保留 `select:focus-visible`）

### P1：低风险重构（视觉改善）
4. 删除第 150-157 行 `.btn` 补丁组
5. 删除第 177-179 行 `.tab` 补丁组
6. 删除第 173-175 行 `.theme-btn` 补丁组

### P2：需确认的设计决策
7. **确认 `.btn--ghost` 默认颜色偏好**：
   - 选项 A（当前实际）：Indigo（`--md-primary`）— 更醒目
   - 选项 B（完整组意图）：Zinc Gray（`--md-sub`）— 更低调
   - 选择后删除对应补丁组

### P3：结构优化
8. 按组件拆分 `app.css`
9. 自定义 dropdown 替换原生 `<select>`（解决原始需求）
10. 提取 HTML/JS 内联样式为语义化 class

---

## 8. 下一步

**请确认以下问题后，我开始执行：**

1. **`.btn--ghost` 默认颜色偏好**：保持当前 Indigo 色，还是改为 Zinc Gray？
2. **执行范围**：先做 P0+P1+P2（清理 + 确认 ghost 颜色），还是同时启动 P3（文件拆分 + dropdown）？
