<div align="center">

# MSP: Media Share & Preview

<img src="web/public/logo.svg" width="120" alt="MSP Logo" />

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp?style=flat-square&color=blue)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp?style=flat-square&color=cyan)
![GitHub license](https://img.shields.io/github/license/blycr/msp?style=flat-square)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp?style=flat-square)
![Codecov](https://img.shields.io/codecov/c/github/blycr/msp?style=flat-square)
[![DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/blycr/msp)

<h3>Your Personal LAN Cinema.</h3>
<p>A lightweight media server for home LAN streaming.</p>

[CodeMap](docs/CodeMap.md) | [中文文档](docs/README_CN.md) | [Report Bug](https://github.com/blycr/msp/issues)

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

> **Note for Firefox users:** Compatibility treatments (GPU layer compositing) have been applied for the audio metadata panel (`audioMeta`). If rendering issues persist, a Chromium-based browser is recommended.

## Playback Behavior

- Direct play is preferred by default.
- Preemptive transcode is applied based on the actual codecs detected inside the file (not the file extension), for higher-risk cases such as:
  - Codecs: `HEVC/H.265`, `AV1`, `VC-1`, `AC-3/E-AC-3`, `DTS`, `TrueHD`, or unknown codecs
- Containers whose codecs cannot be detected (e.g. `AVI`, `WMV`, `WebM`) are served directly first; if direct play fails, MSP retries once, then falls back to transcoding (when enabled).

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
3. Open the URL printed in the console, for example `http://127.0.0.1:8099`. A QR code is printed under each LAN address — scan it with your phone (on the same LAN) to open the page directly.
4. Add shared folders from Settings on first launch.

## Build from Source

Requirements: `Go 1.25+`, `bun 1.3.x` (frontend build), and `PowerShell 7+ (pwsh)` on Windows (`.ps1` scripts auto re-exec from 5.1).  
See [Scripts Guide](scripts/README.md) for detailed build and dev script options.

```bash
git clone https://github.com/blycr/msp.git
cd msp

# Windows (PowerShell)
.\scripts\build.ps1 -P windows              # Windows all architectures
.\scripts\build.ps1 -P all                  # All platforms and architectures
.\scripts\build.ps1 -H                      # Show all available options

# Linux/macOS (Bash)
./scripts/build.sh -P linux                 # Linux all architectures
./scripts/build.sh -P all                   # All platforms and architectures
./scripts/build.sh -h                       # Show all available options
```

For more build and dev script options, see [Scripts Guide](scripts/README.md).

## License

MIT License © 2024-Present [blycr](https://github.com/blycr)

## Acknowledgements

*   [Plyr](https://github.com/sampotts/plyr) - A simple, accessible HTML5 media player.
*   [GORM](https://gorm.io/) - The fantastic ORM library for Golang.
