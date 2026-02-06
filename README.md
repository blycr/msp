# MSP: Media Share & Preview

<div align="center">

<img src="web/public/logo.svg" width="120" alt="MSP Logo" />

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp?style=flat-square&color=blue)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp?style=flat-square&color=cyan)
![GitHub license](https://img.shields.io/github/license/blycr/msp?style=flat-square)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp?style=flat-square)

<h3>Your Personal LAN Cinema.</h3>
<p>A lightweight media server for home LAN streaming.</p>

[中文文档](README_CN.md) | [Wiki](https://github.com/blycr/msp/wiki) | [Report Bug](https://github.com/blycr/msp/issues)

</div>

---

MSP is a single-binary media server focused on practical home use.  
Run it on your PC, share local folders, and play media from any modern browser in your LAN.

## Highlights

- Zero setup: no external database or complex deployment.
- Smart playback: direct play first, transcode only when needed.
- Resume playback: continue from last position across devices.
- Cross-platform server: Windows, Linux, macOS.
- Browser client: desktop and mobile modern browsers.
- Local-first: no cloud account, no tracking.

## Playback Behavior

- Direct play is preferred by default.
- Preemptive transcode is applied only for higher-risk cases, such as:
  - Containers: `AVI`, `WMV`
  - Codecs: `HEVC/H.265`, `VC-1`, `AC-3`, `DTS`, `TrueHD`
- If direct play fails, MSP retries once, then falls back to transcoding (when enabled).

## Preview

<div align="center">

### Video Mode

<kbd>
  <img src="docs/images/preview-video-en.png" alt="Video Mode Preview" width="100%" />
</kbd>

### Audio Mode

<kbd>
  <img src="docs/images/preview-audio-en.png" alt="Audio Mode Preview" width="100%" />
</kbd>

</div>

## Quick Start

1. Download the latest build from [Releases](https://github.com/blycr/msp/releases).
2. Run the executable:
```bash
# Windows
./msp.exe

# Linux/macOS
./msp
```
3. Open the URL printed in the console, for example `http://127.0.0.1:8099`.
4. Add shared folders from Settings on first launch.

## Documentation

- Wiki index: [Project Wiki](https://github.com/blycr/msp/wiki)
- Installation: [Installation Guide](https://github.com/blycr/msp/wiki/Installation)
- Configuration: [Configuration Reference](https://github.com/blycr/msp/wiki/Configuration)
- Playback/Transcode: [Encoding & Transcoding](https://github.com/blycr/msp/wiki/Encoding)
- Security: [Security Guide](https://github.com/blycr/msp/wiki/Security)
- Release: [Release Workflow](https://github.com/blycr/msp/wiki/Release)

## Build from Source

Requirements: `Go 1.24+`, `Node.js 18+` (frontend build)

```bash
git clone https://github.com/blycr/msp.git
cd msp

# Windows
./scripts/build.ps1 -Platforms windows -Architectures x64

# Linux/macOS
./scripts/build.sh --platforms linux --architectures amd64
```

## License

MIT License © 2024-Present [blycr](https://github.com/blycr)

## Acknowledgements

*   [Plyr](https://github.com/sampotts/plyr) - A simple, accessible HTML5 media player.
*   [Gin](https://github.com/gin-gonic/gin) - HTTP web framework written in Go.
*   [GORM](https://gorm.io/) - The fantastic ORM library for Golang.
