# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

[中文文档](README_CN.md)

A fast, privacy-friendly LAN media share with a modern browser player — run one binary, share folders, and start watching/listening instantly.

## Why MSP

- **No cloud upload**: Keep files on your machine; share over LAN with the browser.
- **No heavy media-server setup**: No database tuning; just start and share folders.
- **No client apps to install**: Works on phones/PCs in the same network.
- **Fast to browse**: Categorized lists + search + playlists, designed for large libraries.
- **Playback-first**: Built-in player, image preview, codec/container hints, subtitles/lyrics.
- **Privacy-friendly**: Runtime config stays local; use a template file for sharing configs.

## Quick Start

Run the executable (default port `8099`):

```bash
./bin/windows/x64/msp-windows-amd64.exe
```

After startup, visit the address printed in the console (e.g., `http://127.0.0.1:8099/`).

## Documentation & Help

For configuration, build steps, troubleshooting, and advanced usage, please visit the Project Wiki:

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

### Wiki Highlights
- **[Installation](https://github.com/blycr/msp/wiki/Installation)**: Setup guide for Windows, macOS, and Linux.
- **[Configuration](https://github.com/blycr/msp/wiki/Configuration)**: Detailed config options (Shares, Security, Transcoding).
- **[Encoding Support](https://github.com/blycr/msp/wiki/Encoding)**: Supported formats and FFmpeg transcoding guide.

## License

This project is licensed under the [MIT License](LICENSE).

## Acknowledgements

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player

## Release Notes

- [v0.5.8](docs/release/v0.5.8.md) - Context Refactoring & Security Fixes
- [v0.5.7](docs/release/v0.5.7.md) - Code Refactoring & CI Integration
- [v0.5.6](docs/release/v0.5.6.md)
- [v0.5.5](docs/release/v0.5.5.md)
