# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

一个快速、重视隐私的局域网媒体分享与播放工具：单个可执行文件即可运行，网页端管理共享目录，打开即看即听。

## 为什么用 MSP

- 不用上传网盘：文件留在本机，通过局域网直接在浏览器里看/听
- 不用折腾重型媒体库：不需要复杂部署，启动即可分享目录
- 不用装客户端：同网段手机/电脑打开链接就能用
- 大库也好逛：分类 + 搜索 + 播放列表，适合海量文件浏览
- 播放体验优先：内置播放器与图片预览，提供容器/编码提示，支持字幕/歌词
- 隐私友好：运行时配置只在本地保存，对外分享用示例配置模板

## 快速开始

运行可执行文件（默认端口 `8099`）：

```bash
./bin/windows/x64/msp-windows-amd64.exe
```

启动后访问日志里打印的地址（如 `http://127.0.0.1:8099/`）。

## 文档与帮助

关于配置、构建、常见问题排查与高级用法，请查阅项目 Wiki：

👉 **[MSP Project Wiki (中文文档)](https://github.com/blycr/msp/wiki/Home_CN)**

### Wiki 导航
- **[安装与运行](https://github.com/blycr/msp/wiki/Installation_CN)**: Windows/macOS/Linux 详细部署指南
- **[配置指南](https://github.com/blycr/msp/wiki/Configuration_CN)**: 共享目录、安全设置、转码配置详解
- **[编码兼容性](https://github.com/blycr/msp/wiki/Encoding_CN)**: 格式支持列表与 FFmpeg 转码说明

## 开源许可

本项目采用 [MIT License](LICENSE) 开源。

## 致谢

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player

## 更新日志 (Release Notes)

- [v0.5.8](docs/release/v0.5.8.md) - Context 重构与安全修复
- [v0.5.7](docs/release/v0.5.7.md) - 代码重构与 CI 集成
- [v0.5.6](docs/release/v0.5.6.md)
- [v0.5.5](docs/release/v0.5.5.md)
