# Contributing to MSP

Thank you for your interest in contributing to MSP! We welcome contributions from everyone.

## Development Environment

To contribute to this project, you will need:

*   **Go**: Version 1.27 or higher.
*   **bun**: Version 1.4.x or higher (frontend build).

## Getting Started

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally:
    ```bash
    git clone https://github.com/your-username/msp.git
    cd msp
    ```
3.  **Create a branch** for your feature or fix:
    ```bash
    git checkout -b feat/my-awesome-feature
    ```

## Building the Project

The frontend must be built before `go build` / `go test`, because `web/embed.go` embeds `web/dist/`.

```bash
cd web
bun install
bun run build
cd ..
go test ./...
go build ./cmd/msp
```

### Build notes

- **CGO**: this project is pure Go and releases are built with `CGO_ENABLED=0`. If your machine has
  a C compiler, Go defaults `CGO_ENABLED` to 1 and `go build` produces a *dynamically linked*
  binary that differs from the released artefact. Pin it once with `go env -w CGO_ENABLED=0`.
  The race detector needs cgo, so opt back in explicitly: `CGO_ENABLED=1 go test -race ./...`.
- **Build scripts**: `./scripts/build.sh` (Linux/macOS) and `.\scripts\build.ps1` (Windows) wrap the
  whole target matrix plus tests and lint. Run either with `-h` for options.
- **Network mounts**: don't build or test from a 9p/network path (e.g. `/mnt/*` under WSL). The
  media scanner is file-walk heavy, so `go test` timings stop being meaningful.

## Running Tests

Please ensure all tests pass before submitting a Pull Request.

```bash
# Run all Go tests
go test ./...

# Run linting
golangci-lint run

# Run frontend tests locally — CI does not run these
cd web && bun test
```

CI runs the Go suite with the race detector, `go vet`, `golangci-lint`, and a build across every
supported target. It does **not** run `bun test`, so a failing frontend test will not turn your PR
red — run it yourself before pushing.

## Code Style

*   **Go**: Follow standard Go conventions. We strictly enforce `gofmt`.
*   **JavaScript**: Follow standard ES modules patterns.
*   **Commits**: We follow [Conventional Commits](https://www.conventionalcommits.org/).
    *   `feat: add new media scanner`
    *   `fix: resolve crash on startup`
    *   `docs: update readme`

## Pull Request Process

1.  Ensure your code builds and tests pass.
2.  Update documentation if you are changing behavior or adding features.
3.  Open a Pull Request against the `main` branch.
4.  Provide a clear description of your changes and link to any relevant issues.

## Reporting Issues

If you find a bug or have a feature request, please use the [Issue Tracker](https://github.com/blycr/msp/issues) and select the appropriate template.
