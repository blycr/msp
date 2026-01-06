# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

一个用于局域网内“共享目录 + 浏览器预览播放”的小工具。

## 功能概览

- 局域网访问：自动显示本机可访问 URL（含局域网 IP）
- 共享目录管理：在网页里添加/移除共享目录（Windows 路径自动规范化）
- 分类与列表：视频/音频/图片/其他，支持搜索与播放列表
- 预览播放：视频/音频播放、图片预览
- 编码提示：提供媒体容器/音视频编码探测信息
- PWA：可安装到电脑/手机，以应用方式启动

## 快速开始

运行可执行文件（默认端口 `8099`）：

```bash
./bin/windows/x64/msp-windows-amd64.exe
```

启动后访问日志里打印的地址（如 `http://127.0.0.1:8099/`）。

## 文档与帮助

关于配置、构建、常见问题排查与高级用法，请查阅项目 Wiki：

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

## 开源许可

本项目采用 [MIT License](LICENSE) 开源。

## 致谢

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player
