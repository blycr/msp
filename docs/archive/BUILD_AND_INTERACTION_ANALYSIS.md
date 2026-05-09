# 构建架构与交互阻断问题分析报告

> 生成时间: 2026-05-06
> 状态: **已完成** — 分析完成，修复已执行

---

## 一、问题概述

用户报告两个问题：

1. **交互阻断**: 运行 `bin\windows\amd64\msp-windows-amd64.exe` 时，Web 页面上的交互元素（按钮、输入框等）无法操作；但运行 `bin\windows\x64\msp-windows-amd64.exe` 时一切正常。
2. **架构命名错误**: `bin\windows\x64` 和 `bin\windows\amd64` 本质上是同一种架构（x86-64），不应同时存在。

---

## 二、二进制文件对比分析

### 2.1 文件差异

| 属性 | `bin\windows\amd64\` | `bin\windows\x64\` |
|------|----------------------|---------------------|
| 二进制大小 | 15,314,432 bytes | 15,254,016 bytes |
| 构建时间 | 2026/5/6 15:01:13 | 2026/5/3 3:22:30 |
| Go 版本 | go1.24.4 | go1.24.2 |
| msp.db 大小 | 4,096 bytes (空库) | 1,368,064 bytes (有数据) |
| config.json | `shares: [Music]`, `logFile: ""` | `shares: []`, `logFile: "..."` |

**关键发现**: 两个目录中的二进制文件是**不同时间构建的不同版本**，嵌入了不同版本的前端资源。

### 2.2 日志对比

`amd64` 日志 (2026/5/6):
- 所有 HTTP 请求返回 200/204/206，服务端无异常
- 前端资源哈希: `index-B4N-rLda.js`, `index-CM9OkMuC.css`

`x64` 日志 (2026/3/8):
- 同样所有请求正常
- 前端资源哈希: `index-DFJGSqZD.js`, `index-C-w4n-Rv.css`

**两个二进制嵌入了完全不同的前端资源哈希。**

---

## 三、交互阻断根因分析

### 3.1 最可能的原因: PWA Service Worker 缓存冲突

项目使用 `vite-plugin-pwa` 配置了 Service Worker（[vite.config.js](file:///c:/Users/blycr/msp/web/vite.config.js)）：

```javascript
VitePWA({
  registerType: 'autoUpdate',
  workbox: {
    navigateFallbackDenylist: [/^\/api\//],
    runtimeCaching: [{ urlPattern: /^\/api\//, handler: 'NetworkOnly' }],
  }
})
```

**冲突场景**:

1. 用户先运行 `x64` 二进制（旧版），浏览器通过 Service Worker 缓存了旧版前端资源
2. 用户停止 `x64`，启动 `amd64` 二进制（新版，同一端口 8099）
3. 浏览器导航到 `localhost:8099`，Service Worker 拦截请求
4. **旧版 Service Worker 的 precache manifest 中记录的是旧资源哈希**（如 `index-DFJGSqZD.js`）
5. 新二进制只提供新哈希的资源（如 `index-B4N-rLda.js`），旧哈希资源 404
6. JavaScript 加载失败 → 事件处理器未绑定 → 交互全部失效

验证方法: 在浏览器 DevTools → Application → Service Workers 中检查是否有旧版 SW 残留。

### 3.2 次要可能: 前端 JavaScript 运行时错误

新版前端（含 i18n 修改）可能存在运行时错误，导致 `boot()` 流程中断。

启动流程 ([actions.js:boot()](file:///c:/Users/blycr/msp/web/src/modules/actions.js#L80-L143)):

```
registerSW() → initLang() → initTheme() → bindGlobalHotkeys()
→ bindPinDialog() → bus.emit('boot:init') → bindUI() + updateUIForLang()
→ checkPinRequired() → loadConfig() → loadPrefs() → loadMedia()
```

如果 `bus.emit('boot:init')` 之前任何步骤抛出异常，`bindUI()` 不会执行，所有按钮/输入框的事件监听器都不会注册。

验证方法: 打开浏览器 DevTools → Console，查看是否有红色错误信息。

### 3.3 其他可能原因

- **端口冲突**: 如果两个二进制同时运行在 8099 端口，后启动的会失败
- **数据库状态**: `amd64` 的 msp.db 是空库（4KB），可能影响某些初始化逻辑
- **浏览器缓存**: 非 SW 层面的 HTTP 缓存可能残留旧资源

---

## 四、架构命名问题分析

### 4.1 问题本质

`x64` 和 `amd64` 在 Windows/Go 生态中是**完全相同**的架构：

| 名称 | 说明 |
|------|------|
| `amd64` | AMD 公司首先推出的 64 位 x86 扩展，Go 官方使用此名称 |
| `x64` | Microsoft/Windows 生态中的常用名称 |
| `x86-64` | Intel 的叫法 |
| `Intel 64` | Intel 的另一个名称 |

**Go 编译器只认 `amd64`**，`x64` 不是合法的 `GOARCH` 值。

### 4.2 构建脚本现状

**build.ps1** 和 **build.sh** 中的构建目标:

```
linux/amd64    ✓ 正确
linux/arm64    ✓ 正确
linux/arm      ✓ 正确 (GOARM=7)
darwin/amd64   ✓ 正确
darwin/arm64   ✓ 正确
windows/amd64  ✓ 正确
```

**两个脚本均未定义 `x64` 目标。** `bin\windows\x64\` 目录不是由构建脚本生成的，来源不明（可能是手动复制或旧版脚本遗留）。

### 4.3 release.yml 分析

[.github/workflows/release.yml](file:///c:/Users/blycr/msp/.github/workflows/release.yml) 中的构建矩阵:

| Runner | GOOS/GOARCH | 本地构建脚本对应 |
|--------|-------------|-----------------|
| windows-2019 | windows/amd64 | ✓ build.ps1 有 |
| ubuntu-22.04 | linux/amd64 | ✓ build.ps1 有 |
| ubuntu-22.04 | linux/arm | ✓ build.ps1 有 |
| ubuntu-22.04-linux-arm64 | linux/arm64 | ✓ build.ps1 有 |
| ubuntu-22.04-linux-loong64 | **linux/loong64** | ✗ build.ps1 缺失 |
| macos-13 | darwin/amd64 | ✓ build.ps1 有 |
| macos-latest | darwin/arm64 | ✓ build.ps1 有 |

**发现**: `linux/loong64`（龙芯架构）在 CI/CD 中有构建，但本地构建脚本中缺失。

---

## 五、修复方案

### 5.1 交互阻断修复

**方案 A: 清除 Service Worker 缓存（临时方案）**

在浏览器中手动操作:
1. 打开 DevTools → Application → Service Workers
2. 点击 "Unregister" 注销旧 SW
3. 刷新页面

**方案 B: 在构建脚本中清理旧产物（推荐）**

在 `build.ps1` 和 `build.sh` 的构建流程中，Go 构建前先清理 `web/dist/` 目录，确保嵌入的前端资源是最新的。当前 `vite build` 已配置 `emptyOutDir: true`，所以这一步已覆盖。

**方案 C: 增加 SW 缓存版本管理（根本解决）**

在 `vite.config.js` 的 VitePWA 配置中，添加 `buildAssetsInjection` 或在 `index.html` 中添加 meta 标签强制 SW 更新:

```javascript
// vite.config.js
VitePWA({
  registerType: 'autoUpdate',
  workbox: {
    // 添加: 每次构建生成唯一的 precache manifest
    // 当前已由 Vite 的文件哈希机制保证
    // 但需要确保 SW 更新后旧缓存被清理
    cleanupOutdatedCaches: true, // 已在 sw.js 中启用
  }
})
```

**方案 D: 在前端添加版本检查（根本解决）**

在 `boot()` 函数中添加版本校验，如果检测到 SW 缓存的资源版本与服务端不一致，强制刷新:

```javascript
// actions.js boot() 中
if ('serviceWorker' in navigator) {
  const reg = await navigator.serviceWorker.getRegistration();
  if (reg && reg.active) {
    reg.active.postMessage({ type: 'CHECK_VERSION' });
  }
}
```

**推荐**: 方案 A（用户立即可用）+ 方案 C/D（防止复发）。

### 5.2 架构命名修复

**方案**: 统一使用 `amd64`，删除 `x64` 目录

具体步骤:
1. 确认 `bin\windows\x64\` 中的 `config.json` 和 `msp.db` 是否有用户需要保留的数据
2. 如有需要，将数据迁移到 `bin\windows\amd64\`
3. 删除 `bin\windows\x64\` 目录
4. 在 `.gitignore` 中确认 `bin/` 已被忽略（当前已忽略）

### 5.3 补全 loong64 本地构建支持

在 `build.ps1` 的 `$Profiles` 中添加:

```powershell
@{ Platform = 'linux'; Arch = 'loong64'; OutName = 'msp-linux-loong64'; CC = 'loongarch64-linux-gnu-gcc' }
```

在 `build.sh` 的 `build-linux` 函数中添加:

```bash
log "linux/loong64..."
CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build ...
```

---

## 六、验证步骤

修复后需要验证:

1. [ ] 仅保留 `bin\windows\amd64\` 目录，运行二进制，浏览器访问正常且交互正常
2. [ ] 浏览器 DevTools → Application → Service Workers 中无旧版 SW 残留
3. [ ] 构建脚本 `build.ps1` 和 `build.sh` 运行无错误
4. [ ] `build.ps1 -Clean` 能正确清理所有旧产物
5. [ ] `loong64` 构建配置在两个脚本中均存在
6. [ ] 所有平台二进制文件名符合 `{name}-{os}-{arch}[.exe]` 规范

---

## 七、受影响文件清单

| 文件 | 修改类型 | 说明 |
|------|---------|------|
| `bin\windows\x64\` | 删除 | 冗余目录，与 amd64 同架构 |
| `scripts\build.ps1` | 修改 | 添加 loong64 构建目标 |
| `scripts\build.sh` | 修改 | 添加 loong64 构建目标 |
| `web\vite.config.js` | 可选修改 | 增强 SW 缓存策略 |
| `web\src\modules\actions.js` | 可选修改 | 添加 SW 版本检查逻辑 |
