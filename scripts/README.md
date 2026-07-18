# Build & Dev Scripts

This document describes the build and development scripts for the project. Both PowerShell (`.ps1`) and Bash (`.sh`) versions are provided for cross-platform support.

> Note: the `.ps1` scripts require PowerShell 7+ (`pwsh`). When launched from Windows PowerShell 5.1 they automatically re-execute with `pwsh` if it is installed (some 5.1 environments lack `Get-FileHash`, which `build.ps1` needs for checksums).

[中文版](README_CN.md)

---

## 1. Production Build Scripts (`build.ps1` / `build.sh`)

These scripts run the full production build pipeline: frontend build, backend tests, code checks, cross-compilation, and artifact packaging.

### Parameters

**Windows (PowerShell)**

| Short | Long | Description | Default |
|-------|------|-------------|---------|
| `-P` | `-Preset` | Use a predefined build profile | — |
| `-F` | `-Platforms` | Target platforms, comma-separated | `windows` |
| `-A` | `-Architectures` | Target architectures, comma-separated | `x64` |
| `-T` | `-SkipTests` | Skip Go tests | `false` |
| `-L` | `-SkipLint` | Skip code checks | `false` |
| `-H` | `-Help` | Show help | `false` |
| `-I` | `-ListPresets` | List all available presets | `false` |

**Linux / macOS (Bash)**

| Short | Long | Description | Default |
|-------|------|-------------|---------|
| `-P` | `--preset` | Use a predefined build profile | — |
| `-p` | `--platforms` | Target platforms, comma-separated | `windows` |
| `-a` | `--architectures` | Target architectures, comma-separated | `x64` |
| `-t` | `--skip-tests` | Skip Go tests | `false` |
| `-l` | `--skip-lint` | Skip code checks | `false` |
| `-h` | `--help` | Show help | `false` |
| `-L` | `--list-presets` | List all available presets | `false` |

### Preset System (Recommended)

Presets are predefined platform + architecture combinations managed in `build-profiles.json`. They greatly simplify common build commands.

| Preset | Description | Platforms | Architectures |
|--------|-------------|-----------|---------------|
| `all` | All platforms and architectures | linux, macos, windows, arm | x64, arm64, x86, v7, loong64 |
| `release` | Release build (same as all) | linux, macos, windows, arm | x64, arm64, x86, v7, loong64 |
| `linux` | Linux all architectures | linux | x64, arm64, v7, loong64 |
| `macos` | macOS all architectures | macos | x64, arm64 |
| `darwin` | macOS all architectures (alias) | macos | x64, arm64 |
| `windows` | Windows all architectures | windows | x64, x86 |
| `arm` | ARM all architectures | arm | arm64, v7 |
| `server` | Server deployment (Linux amd64 + arm64) | linux | x64, arm64 |
| `desktop` | Desktop (Windows + macOS) | windows, macos | x64, arm64 |
| `quick` | Quick build (skip tests and lint) | windows | x64 |

You can customize presets by editing `build-profiles.json`.

### Usage Examples

#### Windows (PowerShell)

```powershell
# Show help
.\scripts\build.ps1 -H

# List all presets
.\scripts\build.ps1 -I

# === Preset mode (recommended) ===
.\scripts\build.ps1 -P all                  # All platforms and architectures
.\scripts\build.ps1 -P release              # Release build
.\scripts\build.ps1 -P windows              # Windows only (amd64 + x86)
.\scripts\build.ps1 -P linux                # Linux only (amd64 + arm64 + armv7)
.\scripts\build.ps1 -P server               # Server deployment (Linux amd64 + arm64)
.\scripts\build.ps1 -P desktop              # Desktop (Windows + macOS)
.\scripts\build.ps1 -P quick                # Quick build, skip tests and lint
.\scripts\build.ps1 -P server -T            # Server build, skip tests

# === Custom build ===
.\scripts\build.ps1 -F linux,windows -A x64,arm64   # Comma-separated, no @() needed
.\scripts\build.ps1 -F linux -A x64                 # Single platform, single arch

# === Default build, skip tests and lint ===
.\scripts\build.ps1 -T -L
```

#### Linux / macOS (Bash)

```bash
# Make script executable (first run)
chmod +x ./scripts/build.sh

# Show help
./scripts/build.sh -h

# List all presets
./scripts/build.sh -L

# === Preset mode (recommended) ===
./scripts/build.sh -P all                  # All platforms and architectures
./scripts/build.sh -P release              # Release build
./scripts/build.sh -P windows              # Windows only
./scripts/build.sh -P linux                # Linux only
./scripts/build.sh -P server               # Server deployment
./scripts/build.sh -P desktop              # Desktop
./scripts/build.sh -P quick                # Quick build, skip tests and lint
./scripts/build.sh -P server -t            # Server build, skip tests

# === Custom build ===
./scripts/build.sh -p linux,windows -a x64,arm64

# === Default build, skip tests and lint ===
./scripts/build.sh -t -l
```

### Build Pipeline Details

1.  **Dependency Check**
    - Verifies `go` is installed

2.  **Frontend Build**
    - Checks for `bun`
    - Enters `web/` directory
    - Runs `bun install` if `node_modules` is missing
    - Runs `bun run build` to generate static assets

3.  **Go Test** - skippable via `-T`
    - Runs `go test -v ./...` from project root

4.  **Go Vet** - skippable via `-L`
    - Runs `go vet ./...` for static analysis

5.  **golangci-lint** - skippable via `-L`
    - Runs full lint check if `golangci-lint` is installed
    - Warns and continues if not installed

6.  **Cross Build Artifacts**
    - Sets `GOOS` and `GOARCH` based on target platform + architecture
    - Compiles with `go build -trimpath -ldflags="-s -w"` for optimized binaries
    - **Concurrent builds**: Multiple targets are compiled in parallel (bash auto-detects CPU cores; PowerShell uses 4 parallel jobs)
    - **Output structure**:
        - `bin/<platform>/<arch>/` — compiled binaries
        - `checksums/` — SHA256 checksum files

### Supported Build Targets

| Platform | Architecture | Output Filename |
|----------|-------------|-----------------|
| Linux | amd64 | `msp-linux-amd64` |
| Linux | arm64 | `msp-linux-arm64` |
| Linux | arm (v7) | `msp-linux-armv7` |
| Linux | loong64 | `msp-linux-loong64` |
| macOS | amd64 | `msp-darwin-amd64` |
| macOS | arm64 | `msp-darwin-arm64` |
| Windows | amd64 | `msp-windows-amd64.exe` |
| Windows | 386 | `msp-windows-386.exe` |

---

## 2. Development Scripts (`dev.ps1` / `dev.sh`)

These scripts are designed for local development with full-stack hot-reload support.

### Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `BackendPort` | Backend server listen port | `8099` |

### Startup Flow

1.  **Build Backend**: Compiles Go code to `bin/dev/msp-dev` (or `.exe`)
2.  **Start Backend**:
    - Initializes dev config `bin/dev/config.json` (copies from `config.example.json` or creates empty if missing)
    - Runs backend server on the specified port (default 8099)
    - Sets `MSP_NO_AUTO_OPEN=1` to prevent auto-opening browser
3.  **Start Frontend**:
    - Installs frontend dependencies if needed
    - Starts Vite dev server (`bun run dev`)
    - Sets `MSP_DEV_BACKEND` to proxy API requests to local backend
4.  **Watch Mode**:
    - **Windows (`dev.ps1`)**: Uses .NET `FileSystemWatcher` with 1-second debounce
    - **Linux/macOS (`dev.sh`)**: Polls `.go` file modification timestamps
    - Auto-recompiles and restarts backend on Go code changes
    - Graceful shutdown (5-second timeout before force-kill)

### Interactive Controls

**Windows (PowerShell)**:
- Press `Q` or `Esc` to stop dev server
- Press `R` to manually trigger backend rebuild

**Linux/macOS (Bash)**:
- Press `Ctrl+C` to stop dev server

### Usage Examples

#### Windows (PowerShell)

```powershell
# Start dev server (default port 8099)
.\scripts\dev.ps1

# Custom backend port
.\scripts\dev.ps1 -BackendPort 3000
```

#### Linux / macOS (Bash)

```bash
# Make script executable (first run)
chmod +x ./scripts/dev.sh

# Start dev server (default port 8099)
./scripts/dev.sh

# Custom backend port
./scripts/dev.sh --backend-port 3000
```

### Dev Environment Features

- **Full-stack hot-reload**: Frontend auto-refreshes via Vite; backend auto-restarts on Go changes
- **Graceful shutdown**: Ensures proper resource cleanup (DB connections, log files, etc.)
- **Config isolation**: Uses separate `bin/dev/config.json`, does not affect production config
- **One-command launch**: Manages both frontend and backend processes, cleans up on exit
- **Colored logs**: Different log levels use distinct colors for easy identification

---

## 3. Dependencies

### Required

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Backend compilation |
| bun | 1.3.x | Frontend package manager |
| PowerShell | 7+ (`pwsh`) | Windows `.ps1` scripts (auto re-exec from 5.1; `build.ps1` fails without it, `dev.ps1` falls back to 5.1) |

### Optional

| Tool | Purpose |
|------|---------|
| golangci-lint | Code quality checks |
| jq | Dev config auto-update (dev.sh) |

---

## Summary

| Feature | `build.ps1` / `build.sh` | `dev.ps1` / `dev.sh` |
|---------|--------------------------|----------------------|
| **Purpose** | Production release builds | Local development |
| **Platform** | Cross-platform | Cross-platform |
| **Frontend** | `bun run build` (static) | `bun run dev` (Dev Server) |
| **Backend** | Cross-compile, stripped binaries | Local compile, debug-friendly |
| **Tests** | Runs `go test` | Skipped |
| **Lint** | Runs `go vet` + `golangci-lint` | Skipped |
| **Output** | `bin/`, `checksums/` | `bin/dev/` |
| **Hot Reload** | No | Yes (frontend + backend) |
| **Graceful Shutdown** | N/A | Yes |
