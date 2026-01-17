# Agent Guide for MSP Repository

This document guides autonomous agents working on the MSP (Media Share & Preview) codebase.

## 1. Project Overview

MSP is a local LAN media sharing server written in Go with a Vite-based frontend.
The frontend is embedded into the Go binary at build time.

**Directory Structure:**
```
msp/
├── cmd/msp/          # Main entry point
├── internal/         # Private Go packages
│   ├── config/       # Configuration types and loading
│   ├── db/           # SQLite database layer
│   ├── handler/      # HTTP handlers and middleware
│   ├── media/        # Media scanning and storage
│   ├── server/       # Server lifecycle management
│   ├── types/        # Shared type definitions
│   ├── util/         # Utility functions
│   └── web/          # Embedded web asset serving
├── web/              # Frontend source (Vite + vanilla JS)
│   ├── src/          # Frontend modules
│   └── dist/         # Build output (embedded)
├── scripts/          # Build and dev scripts
├── bin/              # Compiled binaries (per platform)
├── checksums/        # SHA256 checksums for releases
└── debug/            # Debug symbol copies
```

## 2. Build Commands

### Backend (Go)
*   **Language:** Go 1.24+
*   **Quick build:**
    ```bash
    go build ./cmd/msp
    ```
*   **Production build (stripped):**
    ```bash
    go build -trimpath -ldflags="-s -w" -o msp.exe ./cmd/msp
    ```

### Frontend (Vite)
*   **Package Manager:** pnpm (use `corepack enable` if not installed)
*   **Build:**
    ```bash
    cd web
    pnpm install
    pnpm run build
    ```
*   **Dev server:** `pnpm run dev` (proxies `/api` to backend)

### Full Release Build
Use the scripts in `./scripts/`:

**Windows (PowerShell):**
```powershell
./scripts/build.ps1 -Platforms windows -Architectures x64
./scripts/build.ps1 -Platforms windows,linux,macos -Architectures x64,arm64
```

**Linux/macOS (Bash):**
```bash
./scripts/build.sh --platforms linux --architectures amd64
./scripts/build.sh --platforms linux,macos,windows --architectures amd64,arm64
```

These scripts: build frontend, run tests, cross-compile Go, generate checksums.

### Development Mode
```powershell
./scripts/dev.ps1 -BackendPort 8099
```
This watches `.go` files and auto-rebuilds the backend. Frontend runs on Vite dev server.

## 3. Test Commands

### Run All Tests
```bash
go test ./...
```

### Run Single Test
```bash
go test -v ./internal/config -run TestConfigLoad
go test -v ./internal/handler -run TestMiddleware
go test -v ./internal/db -run TestInit
```

### Run Tests in a Package
```bash
go test -v ./internal/handler/...
```

### Run Tests with Coverage
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## 4. Lint Commands

**Linter:** golangci-lint (config in `.golangci.yml`)

```bash
golangci-lint run
```

**Enabled linters:** `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`

## 5. Code Style & Conventions

### Go (Backend)

**Formatting:** Strict `gofmt`. Run `gofmt -w .` before committing.

**Imports:** Group stdlib first, then blank line, then internal/third-party:
```go
import (
    "context"
    "fmt"
    "net/http"

    "msp/internal/config"
    "msp/internal/server"
)
```

**Naming:**
*   Exported: `PascalCase` (e.g., `HandleConfig`, `Config`)
*   Unexported: `camelCase` (e.g., `writeJSON`, `getPort`)
*   Acronyms: `HTTP`, `URL`, `API` (all caps when exported)

**Error Handling:**
*   Always check errors: `if err != nil { ... }`
*   Wrap with context: `fmt.Errorf("loading config: %w", err)`
*   Use `log.Fatal` only at startup; elsewhere return errors
*   Functions prefixed with `Must` may panic (e.g., `MustExeDir`)

**Logging:** Use standard `log` package:
```go
log.Printf("Starting server on port %d", port)
log.Fatal(err)  // Only at startup
```

**Types:** Define in `internal/types/` for shared API types, or locally for package-specific types.

**JSON Tags:** Use `json:"fieldName"` with camelCase field names. Use `omitempty` for optional fields.

### JavaScript/Vite (Frontend)

*   **Style:** ES Modules (`import`/`export`)
*   **Indentation:** 2 spaces
*   **Quotes:** Single quotes preferred
*   **No explicit linter configured** - follow existing patterns

## 6. Testing Strategy

*   **Unit tests:** Place in `*_test.go` next to source files
*   **Table-driven tests:** Preferred pattern for multiple cases
*   **Test data:** Use `internal/handler/test_config.json` as example
*   **Frontend:** No test runner configured; manual verification

## 7. Environment Variables

| Variable | Purpose |
|----------|---------|
| `MSP_NO_AUTO_OPEN` | Set to `1` to disable auto-opening browser on startup |
| `MSP_DEV_BACKEND` | Backend URL for Vite dev proxy (default: `http://127.0.0.1:8099`) |

## 8. API Endpoints

All endpoints under `/api/`:
- `GET/POST /api/config` - Server configuration
- `GET /api/shares` - List configured shares
- `GET /api/media` - List media files
- `GET /api/stream` - Stream media file
- `GET /api/subtitle` - Fetch subtitles
- `GET /api/probe` - Probe media info
- `GET /api/ip` - Get client IP
- `GET/POST /api/prefs` - User preferences
- `GET /api/log` - Server logs
- `POST /api/pin` - PIN authentication

## 9. Cursor/Copilot Rules

No `.cursorrules` or `.github/copilot-instructions.md` found.
Follow standard Go and Vite best practices as documented above.
