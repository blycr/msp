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

## Configuration

MSP uses a JSON configuration file. Key settings include:

- **maxItems**: Set to `0` (recommended) for unlimited incremental scanning powered by SQLite.
- **shares**: Define your media folders.
- **blacklist**: Filter unwanted files/folders.

See `config.example.json` for a full template.

## Documentation & Help

For configuration, build steps, troubleshooting, and advanced usage, please visit the Project Wiki:

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

## License

This project is licensed under the [MIT License](LICENSE).

## Acknowledgements

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player
