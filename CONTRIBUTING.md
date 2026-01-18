# Contributing to MSP

Thank you for your interest in contributing to MSP! We welcome contributions from everyone.

## Development Environment

To contribute to this project, you will need:

*   **Go**: Version 1.24 or higher.
*   **Node.js**: Version 20 or higher.
*   **pnpm**: Enabled via `corepack enable`.

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

### Backend (Go)

```bash
go build ./cmd/msp
```

### Frontend (Vite)

```bash
cd web
pnpm install
pnpm run build
```

## Running Tests

Please ensure all tests pass before submitting a Pull Request.

```bash
# Run all Go tests
go test ./...

# Run linting
golangci-lint run
```

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
