# MSP: 极简局域网媒体服务器

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp?style=flat-square&color=blue)

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp?style=flat-square&color=cyan)

![GitHub license](https://img.shields.io/github/license/blycr/msp?style=flat-square)

![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp?style=flat-square)

![DeepWiki](https://deepwiki.com/badge.svg)

[English](../README.md) | [CodeMap](CodeMap.md) | [提交 Bug](https://github.com/blycr/msp/issues)

---

MSP 是一个单文件部署的媒体服务器，面向家庭场景。

在电脑上启动后，即可通过浏览器在局域网内访问并播放本地媒体。

## 核心亮点

- 零配置启动：无需外部数据库和复杂部署。
- 智能播放：优先直连，必要时再转码。
- 断点续播：跨设备记住播放进度。
- 全平台服务端：支持 Windows、Linux、macOS。
- 浏览器客户端：支持桌面端与移动端现代浏览器。
- 本地优先：不依赖云账号，不做数据追踪。

> **Firefox 用户提示：** 已针对音频信息面板（`audioMeta`）应用 GPU 层兼容性处理（`translateZ(0)`）。如仍出现渲染问题，推荐使用 Chromium 内核浏览器。

## 播放策略

- 默认优先直连播放。
- 预转码基于文件内**实际编码**（而非扩展名）判断，仅对高风险场景应用，例如：
  - 编码：`HEVC/H.265`、`AV1`、`VC-1`、`AC-3/E-AC-3`、`DTS`、`TrueHD`，或无法识别的编码
- 无法检测编码的容器（如 `AVI`、`WMV`、`WebM`）先尝试直连；直连失败时先重试一次，仍失败再回退到转码（需启用转码）。

## 界面预览

### 视频模式

### 音频模式

## 快速开始

1. 从 [Releases 页面](https://github.com/blycr/msp/releases) 下载对应系统版本。
2. 运行可执行文件：

```bash
# Windows
./msp.exe

# Linux/macOS
./msp
```

1. 打开控制台输出地址，例如 `http://127.0.0.1:8099`。局域网地址（如 `http://192.168.x.x:8099/`）下方会打印二维码，手机连同一局域网后扫码即可直达。
2. 首次进入后在设置中添加共享目录。

## 源码编译

环境要求：`Go 1.25+`、`Node.js 18+`（用于构建前端），Windows 还需 `PowerShell 7+（pwsh）`（`.ps1` 脚本可在 5.1 下自动重入）。

详细构建与开发脚本选项，请参阅 [脚本说明](../scripts/README_CN.md)。

```bash
git clone https://github.com/blycr/msp.git
cd msp

# Windows (PowerShell)
.\scripts\build.ps1 -P windows              # Windows 全架构
.\scripts\build.ps1 -P all                  # 全量编译所有平台和架构
.\scripts\build.ps1 -H                      # 查看所有可用选项

# Linux/macOS (Bash)
./scripts/build.sh -P linux                 # Linux 全架构
./scripts/build.sh -P all                   # 全量编译所有平台和架构
./scripts/build.sh -h                       # 查看所有可用选项
```

更多构建与开发脚本选项，请参阅 脚本说明。

## 许可证

本项目采用 [MIT License](../LICENSE) 授权。

## 致谢

- [Plyr](https://github.com/sampotts/plyr) - 简单、灵活的 HTML5 媒体播放器。
- [GORM](https://gorm.io/) - 优秀的 Golang ORM 库。
