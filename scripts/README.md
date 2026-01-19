# 构建与开发脚本说明

本文档详细说明了项目的构建和开发脚本。为了支持跨平台开发，我们提供了 PowerShell (`.ps1`) 和 Bash (`.sh`) 两种版本的构建脚本。

---

## 1. 生产构建脚本 (`build.ps1` / `build.sh`)

这两个脚本用于执行完整的生产环境构建流程，包括前端构建、后端测试、交叉编译以及产物打包。

### 参数说明

这两个脚本接受相同的参数概念，但传递方式略有不同：

- **Platforms**: 目标平台列表。支持 `windows`, `linux`, `macos`, `arm` (指 Linux ARM)。默认: `windows`。
- **Architectures**: 目标架构列表。支持 `x64` (即 amd64), `x86` (即 386), `arm64`, `v7`, `v8`。默认: `x64`。

### 使用示例

#### Windows (PowerShell)

```powershell
# 构建默认平台（Windows x64）
.\scripts\build.ps1

# 构建所有平台
.\scripts\build.ps1 -Platforms @('linux', 'macos', 'windows', 'arm') -Architectures @('x64', 'arm64', 'x86', 'v7', 'v8')

# 仅构建 Linux 平台
.\scripts\build.ps1 -Platforms @('linux') -Architectures @('x64', 'arm64')
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
```

### 构建详细流程

无论使用哪种脚本，构建流程如下：

1.  **Frontend Build (前端构建)**
    - 检查 `pnpm` 是否安装，未安装则尝试使用 `corepack enable`。
    - 进入 `web` 目录。
    - 检查 `node_modules`，不存在则执行 `pnpm install`。
    - 执行 `pnpm run build` 生成静态资源。

2.  **Go Test (后端测试)**
    - 在项目根目录执行 `go test ./...` 运行所有 Go 单元测试。

3.  **Cross Build Artifacts (跨平台交叉编译)**
    - 根据指定的 `Platforms` 和 `Architectures` 组合，设置 `GOOS` 和 `GOARCH` 环境变量。
    - 使用 `go build -trimpath -ldflags="-s -w"` 编译优化后的二进制文件。
    - **输出结构**:
        - `bin/<platform>/<arch>/` : 存放最终的二进制可执行文件。
        - `checksums/` : 存放构建产物的 SHA256 校验和文件。
        - `debug/` : 存放带有符号信息的二进制副本（虽然构建时去除了符号，但此处脚本逻辑保留了文件副本以便后续可能的调试用途或符号恢复，具体视 `go build` 参数而定）。

---

## 2. `dev.ps1` - 开发环境脚本 (Windows)

`dev.ps1` 专为 Windows 环境下的本地开发设计，提供了前后端热重载体验。

### 参数初始化
```powershell
param(
  [int]$BackendPort = 8099  # 默认后端端口
)
```

### 启动流程

1.  **Build Backend**: 编译后端 Go 代码到 `bin/dev/msp-dev.exe`。
2.  **Start Backend**:
    - 初始化开发配置 `bin/dev/config.json` (如果不存在，从 `config.example.json` 复制或创建空配置)。
    - 运行后端服务，监听指定端口 (默认 8099)。
    - 设置环境变量 `MSP_NO_AUTO_OPEN=1` 防止后端自动打开浏览器。
3.  **Start Frontend**:
    - 检查并安装前端依赖。
    - 启动 Vite 开发服务器 (`pnpm run dev`)。
    - 设置 `MSP_DEV_BACKEND` 环境变量指向本地后端，实现前后端代理联调。
4.  **Watch Mode**:
    - 启动文件系统监听器 (FileSystemWatcher)。
    - 监听项目根目录下的 `.go` 文件变化。
    - 一旦检测到 Go 代码修改，自动重新编译并重启后端进程。

### 使用示例

```powershell
# 启动开发环境（默认端口 8099）
.\scripts\dev.ps1

# 指定后端端口启动
.\scripts\dev.ps1 -BackendPort 3000
```

### 开发环境特点

- **全栈热重载**: 修改前端代码 Vite 自动刷新；修改后端 Go 代码自动重启服务。
- **配置隔离**: 开发使用独立的 `bin/dev/config.json`，不影响生产环境配置。
- **一键启动**: 自动管理前后端两个进程，关闭脚本时自动清理残留进程。

---

## 总结

| 特性 | `build.ps1` / `build.sh` | `dev.ps1` |
|------|--------------------------|-----------|
| **用途** | 生产环境发布构建 | 本地开发调试 |
| **平台** | 跨平台 (Windows/Linux/Mac) | Windows Only |
| **前端** | `pnpm run build` (静态构建) | `pnpm run dev` (Dev Server) |
| **后端** | 交叉编译，去除符号表优化体积 | 本地编译，支持 Debug |
| **产物** | `bin/`, `checksums/`, `debug/` | `bin/dev/` |
| **热更新** | 无 | 支持 (前后端双向) |
