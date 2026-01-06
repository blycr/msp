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

## 文档与帮助

关于配置参数、构建步骤、常见问题（如视频无法播放的编码问题）及更多高级用法，请查阅项目 Wiki：

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

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
