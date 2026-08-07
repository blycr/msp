# AGENTS.md — MSP (Media Share & Preview)

## Build & Test

Frontend must be built before any Go compile or test, because `web/embed.go` embeds `web/dist/`.

```bash
cd web && bun install && bun run build
go test ./... && go vet ./... && golangci-lint run
```

Lint-only stubs (no real frontend build):
```bash
mkdir -p web/dist
echo "DUMMY" > web/dist/index.html
echo "DUMMY" > web/dist/manifest.webmanifest
```

## Dev Commands

| Platform | Dev server | Production build |
|----------|-----------|------------------|
| Windows | `.\scripts\dev.ps1` | `.\scripts\build.ps1` |
| Linux/Mac | `./scripts/dev.sh` | `./scripts/build.sh` |

Both set `MSP_NO_AUTO_OPEN=1`. Build options: `-P` profiles, `-T` skip tests, `-L` skip lint. See `-h`.

## Architecture

- **Go 1.25**. Entrypoint: `cmd/msp/main.go` (HTTP `:8099`).
- **Embed**: `web/embed.go` → `//go:embed dist`.
- **Config**: `<exe_dir>/config.json`, hot-reloaded. Template: `config.example.json`.
- **DB**: SQLite (`glebarez/sqlite`, pure Go, no CGO). File: `<exe_dir>/msp.db`.

| Package | Responsibility |
|---------|---------------|
| `internal/cache` | Media cache management |
| `internal/config` | Config parsing & validation |
| `internal/constants` | Shared constants & errors |
| `internal/domain` | Domain types (Share, etc.) |
| `internal/handler` | HTTP handlers & middleware |
| `internal/media` | Media processing, transcoder, HW accel |
| `internal/scanner` | File system scanner & subtitle detection |
| `internal/server` | Server orchestration & config mgmt |
| `internal/service` | Business logic services |
| `internal/storage` | SQLite/GORM store & interfaces |
| `internal/types` | Shared type definitions |
| `internal/util` | Utilities |
| `internal/web` | Embedded web serving logic |
| `web/` | Frontend (Vite 8 + vanilla JS + PWA, bun 1.3) |
| `web/src/modules/` | JS modules: player, playlist, api, state, ui, i18n |
| `web/src/styles/` | Componentized CSS |

## Env & Frontend

| Variable | Purpose |
|----------|---------|
| `MSP_NO_AUTO_OPEN` | `1` = prevent auto-opening browser |
| `MSP_DEV_BACKEND` | Vite proxy target (default `http://127.0.0.1:8099`) |

- No frontend test runner. `web/web_test.go` only verifies embed FS compiles.
- Vite dev proxy: `/api` → `MSP_DEV_BACKEND`.

## CI / Release / Docker

- **Lint**: `.golangci.yml` v2 (errcheck, govet, ineffassign, staticcheck, unused, gosec, gocyclo, misspell). Timeout 5m.
- **CI**: `check.yml` (test, lint, build-check). `release.yml` on `v*` tags. Release notes from `docs/release/<tag>.md`.
- **Docker**: Multi-stage (node:22-alpine → golang:1.25-alpine → alpine). CGO_ENABLED=0. Volumes: `/data`, `/media`.
- **Release checklist**: Update `CHANGELOG.md` → create `docs/release/vX.Y.Z.md` → verify `bun run build && go build` → commit → tag → push.
- **Common release mistakes**: forgetting release notes file; tagging before commit; skipping frontend build before `go build`.

## Conventions

- Go module name is `msp` (not a URL path).
- `config.json` and `msp.db` are gitignored; only `config.example.json` is tracked.

---

## Decision Protocol

### Step 1: Assess complexity

| Level | Signal | Action |
|-------|--------|--------|
| **Nano** | Single-file details (fields, comments, format, constants, simple rename) | Do it. Log `[Nano: X, reason]` |
| **Light** | ≤2 files, ≤3 steps, no interface change | 1-line plan, then do. Log `[Light]` |
| **Standard** | 3–5 files, or interface change, or known risk | Plan (steps + risks), **wait for approval**, then execute |
| **Strict** | >5 files, architecture, migration, security | Full plan + challenge, **mandatory approval**, stepwise execution |

**Nano escalation**: if a "simple" change cascades, stop. Log `[Nano escalated: reason]`, switch to Standard.

### Step 2: Execute

- **Follow the plan**. If deviating:
  - Standard/Strict: `[Alert: reason]` + pause for direction
  - Light: `[Tweak: reason]` + continue
- **Uncertain?** Mark it:
  - `[High-confidence]` → proceed
  - `[Medium-confidence]` → proceed but log it, confirm at next checkpoint
  - `[Low-confidence]` → block, do not guess
- **Major change of mind?** Leave a trace: `~~old~~` `[Pivot: reason]` `[New approach]` `[Impact]`
- **Local adjustment?** `[Tweak: X→Y, reason]`
- **Stuck in a loop?** Direction changes ≥3 times → `[Deadlock] Options: A/B/C [Evidence] [Request: pick one]`

### Step 3: Handover (only at phase transitions)

When moving from Plan → Execute, or Execute → Review, summarize in one block:
```
[Handover] Done: X | Next: Y | Risks: Z | Rollback: W
```
Do not hand over within a phase.

### Safety overrides

- Data security / production / privacy: **strict process, no downgrade**, even under time pressure.
- User says "just do it": skip Plan, but log `[Simplified: user override]`.
- Time pressure: `[Iterative: vX, known defect Y]` is acceptable.

---

## Evolution Log

**Write only when a lesson is worth remembering across sessions.** Do not append after every task.

**Triggers**: user corrects me; CI/build fails for non-obvious reason; I discover a constraint the hard way; a repeatable pattern emerges.

**Do not write**: one-off typos; temporary glitches; obvious syntax errors; hunches without evidence.

**Format**: `[YYYY-MM-DD] Trigger → Correct action [Context]`

**Maintenance**: if >15 entries, consolidate oldest or archive to `docs/archive/AGENTS_EVOLUTION.md`.

### Entries
<!-- Append below. -->
[2026-07-13] User correction → use bun/bunx/uv instead of npx/npm/pip; prefer a domestic registry and use SOCKS5 7890 only for network fallback [Tooling]
[2026-07-18] build.ps1 failed at checksum step → this machine's Windows PowerShell 5.1 lacks Get-FileHash; build.ps1/dev.ps1 now auto re-exec into pwsh (PS7) when PS < 7 [Build]
[2026-08-07] go get/tidy on proxy.golang.org times out → use `GOPROXY=https://goproxy.cn,direct` for Go dependency fetches (same domestic-registry rule as bun/npm) [Tooling]
