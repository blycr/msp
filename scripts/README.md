# 构建与开发脚本说明

本文档详细说明了项目的构建和开发脚本。为了支持跨平台开发，我们提供了 PowerShell (`.ps1`) 和 Bash (`.sh`) 两种版本的构建脚本。

---

## 1. 生产构建脚本 (`build.ps1` / `build.sh`)

这两个脚本用于执行完整的生产环境构建流程，包括前端构建、后端测试、代码检查、交叉编译以及产物打包。

### 参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `Platforms` | 目标平台列表。支持 `windows`, `linux`, `macos`, `arm` | `windows` |
| `Architectures` | 目标架构列表。支持 `x64`, `x86`, `amd64`, `arm64`, `v7`, `v8` | `x64` |
| `SkipTests` | 跳过 Go 测试 | `false` |
| `SkipLint` | 跳过代码检查 (go vet, golangci-lint) | `false` |

### 使用示例

#### Windows (PowerShell)

```powershell
# 构建默认平台（Windows x64）
.\scripts\build.ps1

# 构建所有平台
.\scripts\build.ps1 -Platforms @('linux', 'macos', 'windows', 'arm') -Architectures @('x64', 'arm64', 'x86', 'v7', 'v8')

# 仅构建 Linux 平台
.\scripts\build.ps1 -Platforms @('linux') -Architectures @('x64', 'arm64')

# 跳过测试和 lint 检查（快速构建）
.\scripts\build.ps1 -SkipTests -SkipLint
```

#### Linux / macOS (Bash)

```bash
# 给脚本添加执行权限（首次运行）
chmod +x ./scripts/build.sh

# 构建默认平台（Windows x64）
./scripts/build.sh

# 构建所有平台 (注意：参数使用逗号分隔的字符串)
./scripts/build.sh --platforms "linux,macos,windows,arm" --architectures "x64,arm64,x86,v7,v8"

# 仅构建 Linux 平台
./scripts/build.sh --platforms "linux" --architectures "x64,arm64"

# 跳过测试和 lint 检查（快速构建）
./scripts/build.sh --skip-tests --skip-lint
```

### 构建详细流程

1.  **依赖检查**
    - 检查 `go` 是否已安装

2.  **Frontend Build (前端构建)**
    - 检查 `pnpm` 是否安装，未安装则尝试使用 `corepack enable`
    - 进入 `web` 目录
    - 检查 `node_modules`，不存在则执行 `pnpm install`
    - 执行 `pnpm run build` 生成静态资源

3.  **Go Test (后端测试)** - 可通过 `-SkipTests` 跳过
    - 在项目根目录执行 `go test -v ./...` 运行所有 Go 单元测试

4.  **Go Vet (静态分析)** - 可通过 `-SkipLint` 跳过
    - 执行 `go vet ./...` 进行静态代码分析

5.  **golangci-lint (代码检查)** - 可通过 `-SkipLint` 跳过
    - 如果安装了 `golangci-lint`，执行完整代码检查
    - 未安装时会发出警告但继续构建

6.  **Cross Build Artifacts (跨平台交叉编译)**
    - 根据指定的 `Platforms` 和 `Architectures` 组合，设置 `GOOS` 和 `GOARCH` 环境变量
    - 使用 `go build -trimpath -ldflags="-s -w"` 编译优化后的二进制文件
    - **输出结构**:
        - `bin/<platform>/<arch>/` : 存放最终的二进制可执行文件
        - `checksums/` : 存放构建产物的 SHA256 校验和文件

### 支持的构建目标

| 平台 | 架构 | 输出文件名 |
|------|------|------------|
| Linux | amd64 | `msp-linux-amd64` |
| Linux | arm64 | `msp-linux-arm64` |
| Linux | arm (v7) | `msp-linux-armv7` |
| macOS | amd64 | `msp-darwin-amd64` |
| macOS | arm64 | `msp-darwin-arm64` |
| Windows | amd64 | `msp-windows-amd64.exe` |
| Windows | 386 | `msp-windows-386.exe` |

---

## 2. 开发环境脚本 (`dev.ps1` / `dev.sh`)

这两个脚本专为本地开发设计，提供了前后端热重载体验。

### 参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `BackendPort` | 指定后端服务监听的端口 | `8099` |

### 启动流程

1.  **Build Backend**: 编译后端 Go 代码到 `bin/dev/msp-dev` (或 `.exe`)
2.  **Start Backend**:
    - 初始化开发配置 `bin/dev/config.json` (如果不存在，从 `config.example.json` 复制或创建空配置)
    - 运行后端服务，监听指定端口 (默认 8099)
    - 设置环境变量 `MSP_NO_AUTO_OPEN=1` 防止后端自动打开浏览器
3.  **Start Frontend**:
    - 检查并安装前端依赖
    - 启动 Vite 开发服务器 (`pnpm run dev`)
    - 设置 `MSP_DEV_BACKEND` 环境变量指向本地后端，实现前后端代理联调
4.  **Watch Mode**:
    - **Windows (`dev.ps1`)**: 使用 .NET `FileSystemWatcher` 监听文件变化，带 1 秒防抖
    - **Linux/macOS (`dev.sh`)**: 轮询检查 `.go` 文件的修改时间戳
    - 一旦检测到 Go 代码修改，自动重新编译并重启后端进程
    - 支持优雅关闭（等待最多 5 秒，超时强制终止）

### 交互控制

**Windows (PowerShell)**:
- 按 `Q` 或 `Esc` 停止开发服务器
- 按 `R` 手动触发后端重建

**Linux/macOS (Bash)**:
- 按 `Ctrl+C` 停止开发服务器

### 使用示例

#### Windows (PowerShell)

```powershell
# 启动开发环境（默认端口 8099）
.\scripts\dev.ps1

# 指定后端端口启动
.\scripts\dev.ps1 -BackendPort 3000
```

#### Linux / macOS (Bash)

```bash
# 给脚本添加执行权限（首次运行）
chmod +x ./scripts/dev.sh

# 启动开发环境（默认端口 8099）
./scripts/dev.sh

# 指定后端端口启动
./scripts/dev.sh --backend-port 3000
```

### 开发环境特点

- **全栈热重载**: 修改前端代码 Vite 自动刷新；修改后端 Go 代码自动重启服务
- **优雅关闭**: 后端进程支持优雅关闭，确保资源正确释放（数据库连接、日志文件等）
- **配置隔离**: 开发使用独立的 `bin/dev/config.json`，不影响生产环境配置
- **一键启动**: 自动管理前后端两个进程，关闭脚本时自动清理残留进程
- **彩色日志**: 不同级别的日志使用不同颜色显示，便于区分

---

## 3. 依赖要求

### 必需依赖

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.24+ | 后端编译 |
| pnpm | 10.x | 前端包管理 |

### 可选依赖

| 工具 | 用途 |
|------|------|
| golangci-lint | 代码质量检查 |
| jq | 开发配置自动更新 (dev.sh) |

---

## 总结

| 特性 | `build.ps1` / `build.sh` | `dev.ps1` / `dev.sh` |
|------|--------------------------|----------------------|
| **用途** | 生产环境发布构建 | 本地开发调试 |
| **平台** | 跨平台 (Windows/Linux/Mac) | 跨平台 (Windows/Linux/Mac) |
| **前端** | `pnpm run build` (静态构建) | `pnpm run dev` (Dev Server) |
| **后端** | 交叉编译，去除符号表优化体积 | 本地编译，支持 Debug |
| **测试** | 执行 `go test` | 不执行 |
| **Lint** | 执行 `go vet` 和 `golangci-lint` | 不执行 |
| **产物** | `bin/`, `checksums/` | `bin/dev/` |
| **热更新** | 无 | 支持 (前后端双向) |
| **优雅关闭** | 不适用 | 支持 |
