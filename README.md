# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

[中文文档](README_CN.md)

A lightweight tool for "Local Directory Sharing + Browser Preview/Playback" within a LAN. The backend uses Go to provide file indexing and streaming interfaces, while the frontend uses pure static pages for video/audio/image preview and playlist management.

## Features Overview

- **LAN Access**: Automatically displays accessible URLs (including LAN IP).
- **Share Management**: Add/remove shared directories via the web interface.
- **Categorization**: Video, Audio, Image, Other; supports search and playlists.
- **Preview & Play**: Video/Audio player (with speed control, lyrics), Image gallery.
- **Encoding Hints**: Detects media container and codec information.

## Quick Start

Simply run the executable (default port `8099`):

```bash
./msp.exe
```

After startup, visit the address printed in the console (e.g., `http://127.0.0.1:8099/`).

## Documentation & Help

For configuration parameters, build steps, common issues (e.g., video playback encoding support), and advanced usage, please visit the Project Wiki:

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

## Contribution & Development

- **Runtime Config**: `config.json` (auto-generated on first run).
- **Build**: Requires Go 1.18+.

## License

This project is licensed under the [MIT License](LICENSE).

## Acknowledgements

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player
