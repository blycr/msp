# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

一个用于局域网内“共享目录 + 浏览器预览播放”的小工具：后端用 Go 提供文件索引与流式读取接口，前端用纯静态页面实现视频/音频/图片预览与播放列表。

## 功能概览

- 局域网访问：自动显示本机可访问 URL（含局域网 IP）
- 共享目录管理：在网页里添加/移除共享目录（Windows 路径自动规范化）
- 分类与列表：视频/音频/图片/其他，支持搜索与播放列表
- 预览播放：视频/音频播放、图片预览
- 编码提示：提供媒体容器/音视频编码探测信息，用于定位“同为 mp4/mkv 但有的能播有的不能播”的原因

## 运行

直接运行可执行文件（默认端口 `8099`）：

```bash
./msp.exe
```

启动后访问日志里打印的地址，例如：

- http://127.0.0.1:8099/
- http://<你的局域网IP>:8099/

## 配置（config.json）

`config.json` 是运行时配置文件（包含你本机的共享目录绝对路径），因此不建议提交到仓库。

- 首次运行：如果当前目录不存在 `config.json`，程序会自动生成一个默认配置
- 配置共享目录：通过网页中的“共享目录设置”添加/移除共享目录，程序会自动写回 `config.json`
- 配置示例：可参考仓库内的 `config.example.json`

关键字段：

- `port`：监听端口
- `shares`：共享目录列表（`label` 为别名，`path` 为目录路径）
- `features`：前端能力开关
- `playback`：各类媒体的播放行为配置

## 构建（Windows）

需要 Go（`go 1.22+`）。

```powershell
.\scripts\build.ps1
```

构建完成后生成 `msp.exe`（已在 `.gitignore` 中忽略，不会提交到仓库）。

## 项目结构

- `cmd/msp`：可执行入口
- `web/static`：前端静态资源（`index.html`、`app.js`、`app.css`、`assets/...`），由 Go 通过 embed 内置
- `scripts`：构建脚本
- `CHANGELOG.md`：版本变更记录

## 开发说明

为了保持仓库整洁与安全，以下文件/目录已被 `.gitignore` 忽略，请勿强制提交：

- **运行时配置**：`config.json`（请使用 `config.example.json` 复制修改）
- **IDE/编辑器配置**：`.trae`, `.vscode`, `.idea` 等
- **部署/平台配置**：`.vercel`, `vercel.json` 等
- **本地文档/草稿**：`docs/`

## 编码兼容性说明

“同样是 .mp4/.mkv 容器，但有的能播有的不能播”通常是因为容器内的编码不同：

- 视频：H.264 通常兼容性最好；HEVC/H.265、AV1 取决于浏览器/设备硬解与系统支持
- 音频：AAC 通常兼容性好；AC-3/E-AC-3 在浏览器里经常不支持

本项目会在前端展示探测到的容器、视频编码与音频编码信息，帮助快速判断不可播放的根因。

## 安全提示

- 仅会对 `shares` 中配置的目录开放访问；不在共享目录内的文件会被拒绝。
- 建议仅在可信局域网内使用，不建议直接暴露到公网。

## 开源许可

本项目采用 [MIT License](LICENSE) 开源。

## 致谢

本项目使用或参考了以下开源项目，感谢开发者的无私奉献：

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player for Video, Audio, YouTube and Vimeo

