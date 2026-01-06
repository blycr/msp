# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

一个用于局域网内“共享目录 + 浏览器预览播放”的小工具：后端用 Go 提供文件索引与流式读取接口，前端基于 Vite 构建，提供现代化的用户体验。

## 功能概览

- 局域网访问：自动显示本机可访问 URL（含局域网 IP）
- 共享目录管理：在网页里添加/移除共享目录（Windows 路径自动规范化）
- 分类与列表：视频/音频/图片/其他，支持搜索与播放列表
- 预览播放：视频/音频播放、图片预览
- 编码提示：提供媒体容器/音视频编码探测信息
- 现代化架构：模块化 Go 后端 + Vite 前端工程化

## 更新亮点（v0.5.0）

- **PWA 支持**：可安装到电脑桌面或手机主屏幕，界面支持离线壳，以独立应用形式运行。
- **主题切换更顺滑**：明暗模式切换采用轻量淡入动画，并兼顾无障碍（尊重“减少动态效果”系统设置）。
- **音频/图片淡入**：音频播放器与图片预览在切换时使用淡入过渡，避免顿挫。
- **列表分页**：左侧文件列表与播放列表默认分页（每页 10 项），减少一次性渲染带来的卡顿。

## PWA 使用方法

- 电脑端（Chrome/Edge）：地址栏会出现“安装”图标，点击即可安装为应用。
- Android（Chrome）：菜单 → “安装应用”或“添加到主屏幕”。
- iOS（Safari）：分享 → “添加到主屏幕”。

## 快速开始

直接运行可执行文件（默认端口 `8099`）：

```bash
./msp.exe
```

启动后访问日志里打印的地址（如 `http://127.0.0.1:8099/`）。

## 从源码构建

- 环境要求：
  - Go 1.22+
  - Node.js 18+
- 前端：
  - `cd web && npm install && npm run build`
- 后端：
  - `go build ./cmd/msp`
- 一键构建（Windows）：
  - `scripts/build.ps1`

### 跨平台构建（Go 原生）

- 默认：Windows x64
  - `scripts/build.ps1`
- 可选平台/架构：
  - `scripts/build.ps1 -Platforms windows,linux,macos,arm -Architectures x64,x86,amd64,arm64,v7,v8`
- 产物目录：
  - `bin/windows/x64/msp-windows-amd64.exe`
  - `bin/windows/x86/msp-windows-386.exe`
  - `bin/linux/amd64/msp-linux-amd64`
  - `bin/linux/arm64/msp-linux-arm64`
  - `bin/arm/v7/msp-arm-v7`
  - `bin/arm/v8/msp-arm-v8`
  - `bin/macos/msp-macos-amd64`, `bin/macos/msp-macos-arm64`
  - 校验在 `checksums/`，调试拷贝在 `debug/`
## 文档与帮助

关于配置参数、构建步骤、常见问题（如视频无法播放的编码问题）及更多高级用法，请查阅项目 Wiki：

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

## 项目优势

- 简洁高效：零配置启动，立即共享并在浏览器预览
- 现代体验：前端基于 Vite，响应式布局与顺滑过渡
- 实用功能：播放列表、字幕、编码提示、图片预览
- 重视隐私：运行时配置仅本地保存，提供示例模板
- 轻量可靠：模块化 Go 后端，资源占用低

## 贡献与开发

- **运行时配置**：`config.json`（请使用 `config.example.json` 复制修改，不要提交）
- **构建**：
    - **Go**: 需要 1.22+
    - **Node.js**: 需要用于构建前端资源
    - 推荐使用 `scripts/build.ps1` (Windows) 进行一键全栈构建
- **隐私提醒**：请勿将私密配置提交到远程仓库，使用 `config.example.json` 作为模板。

## 开源许可

本项目采用 [MIT License](LICENSE) 开源。

## 致谢

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player
