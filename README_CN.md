# MSP: 极简局域网媒体服务器

<div align="center">

<img src="web/public/logo.svg" width="120" alt="MSP Logo" />

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp?style=flat-square&color=blue)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp?style=flat-square&color=cyan)
![GitHub license](https://img.shields.io/github/license/blycr/msp?style=flat-square)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp?style=flat-square)

<h3>打造你的家庭局域网影院。</h3>
<p>轻量、高速、隐私安全的媒体流服务，专为家庭网络设计。</p>

[English](README.md) | [Wiki 文档](https://github.com/blycr/msp/wiki) | [提交 Bug](https://github.com/blycr/msp/issues)

</div>

---

**MSP** 是一个单文件部署的媒体服务器。只需在电脑上运行它，即可立刻通过现代化的 Web 界面，在局域网内的任何设备（手机、平板、电视）上播放你的视频和音频收藏。

## 核心特性

| 功能 | 说明 |
| :--- | :--- |
| **零配置启动** | 无需安装数据库，无需复杂的环境配置。下载即用，一键运行。 |
| **智能转码** | 自动检测并实时转码浏览器不支持的格式（如 MKV, FLAC, AVI），实现无缝播放。 |
| **断点续播** | 自动记录播放进度，在不同设备间无缝切换，随时继续观看。 |
| **全平台支持** | 服务端支持 Windows/Linux/macOS。客户端支持所有现代浏览器（移动端适配完美）。 |
| **隐私优先** | 数据完全保存在本地，不上传云端，无追踪，安全可靠。 |
| **极速体验** | 基于 Go 和 Vite 构建。秒级启动，瞬间扫描海量媒体库。 |

## 界面预览

<div align="center">

### 视频模式

<kbd>
  <img src="docs/images/preview-video-cn.png" alt="视频模式预览" width="100%" />
</kbd>

### 音频模式

<kbd>
  <img src="docs/images/preview-audio-cn.png" alt="音频模式预览" width="100%" />
</kbd>

</div>

## 快速开始

1.  **下载** 对应系统的最新版本：[Releases 页面](https://github.com/blycr/msp/releases)。
2.  **运行** 可执行文件：
    ```bash
    # Windows
    ./msp.exe

    # Linux/macOS
    ./msp
    ```
3.  **打开浏览器**：
    控制台会打印访问地址（例如 `http://127.0.0.1:8099`）。
    *首次运行时，你可以在网页界面中直接添加需要共享的文件夹。*

## 文档支持

更多高级用法，请查阅 **[项目 Wiki](https://github.com/blycr/msp/wiki)**：

*   **[安装指南](https://github.com/blycr/msp/wiki/Installation)** (包含 Docker、服务化运行教程)
*   **[配置详解](https://github.com/blycr/msp/wiki/Configuration)**
*   **[编码与转码](https://github.com/blycr/msp/wiki/Encoding)** (支持的格式说明)

## 源码编译

编译环境要求：**Go 1.24+**, **Node.js 18+** (用于编译前端)

```bash
# 克隆仓库
git clone https://github.com/blycr/msp.git
cd msp

# 编译所有组件 (前端 + 后端)
# Windows 用户
./scripts/build.ps1 -Platforms windows -Architectures x64

# Linux/macOS 用户
./scripts/build.sh --platforms linux --architectures amd64
```

## 许可证

本项目采用 [MIT License](LICENSE) 授权。

## 致谢

*   [Plyr](https://github.com/sampotts/plyr) - 简单、灵活的 HTML5 媒体播放器。
*   [Gin](https://github.com/gin-gonic/gin) - 高性能 Go Web 框架。
*   [GORM](https://gorm.io/) - 优秀的 Golang ORM 库。
