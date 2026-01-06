# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

[中文文档](README_CN.md)

A lightweight tool for "Local Directory Sharing + Browser Preview/Playback" within a LAN. The backend uses Go to provide file indexing and streaming interfaces, while the frontend is built with Vite for a modern, responsive user experience.

## Features Overview

- **LAN Access**: Automatically displays accessible URLs (including LAN IP).
- **Share Management**: Add/remove shared directories via the web interface.
- **Categorization**: Video, Audio, Image, Other; supports search and playlists.
- **Preview & Play**: Video/Audio player (with speed control, lyrics), Image gallery.
- **Encoding Hints**: Detects media container and codec information.
- **Modern Architecture**: Modular Go backend + Vite-powered Frontend.

## What’s New (v0.5.3)

- **Faster Startup**: `GET /api/media?limit=200` for first paint, then load full list in background.
- **Better Caching**: API supports `ETag/If-None-Match` (304); stale-while-revalidate memory cache.
- **Richer Probe**: `GET /api/probe` returns container/codec + sidecar subtitles list.
- **Local Media Cache**: runtime generates `config.json.media_cache.json` (do not commit).

## PWA Usage

- Desktop (Chrome/Edge): Click the “Install” icon in the address bar to add MSP as an app.
- Android (Chrome): Menu → “Install app” or “Add to Home screen”.
- iOS (Safari): Share → “Add to Home Screen”.

## Quick Start

Run the built executable (default port `8099`):

```bash
./bin/windows/x64/msp-windows-amd64.exe
```

After startup, visit the address printed in the console (e.g., `http://127.0.0.1:8099/`).

## Build from Source

- Requirements:
  - Go 1.22+
  - Node.js 18+
- Frontend:
  - `cd web && npm install && npm run build`
- Backend:
  - `go build ./cmd/msp`
- One-step (Windows):
  - `scripts/build.ps1`
- One-step (Linux/macOS):
  - `scripts/build.sh`

### Cross-Platform Build (Go Native)

- Default: Windows x64
  - `scripts/build.ps1` or `scripts/build.sh`
- Select platforms/architectures:
  - `scripts/build.ps1 -Platforms windows,linux,macos,arm -Architectures x64,x86,amd64,arm64,v7,v8`
  - `scripts/build.sh -Platforms windows,linux,macos,arm -Architectures x64,x86,amd64,arm64,v7,v8`
- Outputs:
  - `bin/windows/x64/msp-windows-amd64.exe`
  - `bin/windows/x86/msp-windows-386.exe`
  - `bin/linux/amd64/msp-linux-amd64`
  - `bin/linux/arm64/msp-linux-arm64`
  - `bin/arm/v7/msp-arm-v7`
  - `bin/arm/v8/msp-arm-v8`
  - `bin/macos/msp-macos-amd64`, `bin/macos/msp-macos-arm64`
  - Checksums in `checksums/`, debug copies in `debug/`

## Documentation & Help

For configuration parameters, build steps, common issues (e.g., video playback encoding support), and advanced usage, please visit the Project Wiki:

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

## Why MSP

- Simple and Fast: zero-config startup, immediate LAN sharing and preview
- Modern UX: Vite-built frontend, responsive layout, smooth transitions
- Practical Features: playlists, subtitles, codec hints, image preview
- Privacy-Friendly: runtime config stays local; templates provided
- Lightweight: modular Go backend, minimal resource usage

## Contribution & Development
 
- **Runtime Config**: `config.json` (auto-generated on first run).
- **Build**: 
    - **Go**: 1.22+ required.
    - **Node.js**: Required for building frontend assets.
    - Use `scripts/build.ps1` (Windows) for a one-step full stack build.
- **Privacy Note**: Do not commit private configs. Use `config.example.json` as a template.

## License

This project is licensed under the [MIT License](LICENSE).

## Acknowledgements

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player
