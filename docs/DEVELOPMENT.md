# 开发指南 (Development Guide)

欢迎参与 MSP (Media Share & Preview) 项目开发！本项目采用前后端分离架构（构建时合并），后端使用 Go，前端使用 Vue 3。

## 🛠️ 技术栈

*   **后端**: Go 1.24+ (Gin, GORM, SQLite)
*   **前端**: Vue 3, TypeScript, Vite, Pinia
*   **构建**: Shell/PowerShell 脚本自动化
*   **包管理**: pnpm (前端)

## ⚡ 环境要求

在开始之前，请确保您的环境满足以下要求：

*   **Go**: 1.24 或更高版本
*   **Node.js**: 18.0 或更高版本 (推荐 v20 LTS)
*   **pnpm**: 必须使用 pnpm 管理前端依赖 (运行 `corepack enable` 启用)
*   **GCC**: 用于编译 CGO 依赖 (SQLite) - *Windows 用户建议安装 TDM-GCC*

## 🚀 快速开始

### 1. 初始化依赖

```bash
# 后端依赖
go mod download

# 前端依赖
cd web
pnpm install
cd ..
```

### 2. 启动开发模式 (热重载)

我们提供了便捷的脚本，可同时启动后端服务（监听文件变化）和前端 Vite 开发服务器。

**Windows (PowerShell):**
```powershell
./scripts/dev.ps1
```

**Linux / macOS:**
目前建议手动在两个终端分别运行：
```bash
# 终端 1: 后端
go run ./cmd/msp

# 终端 2: 前端
cd web
export MSP_DEV_BACKEND="http://127.0.0.1:8099"
pnpm dev
```

*默认端口：后端 `:8099`，前端 `:5173`。访问 `http://localhost:5173` 进行开发。*

### 3. 生产构建

构建生产环境的单文件二进制程序（自动嵌入前端资源）。

**Windows:**
```powershell
./scripts/build.ps1
```

**Linux / macOS:**
```bash
./scripts/build.sh
```

构建产物位于 `bin/` 目录下。

## 📂 目录结构

```
msp/
├── bin/              # 构建产出 (gitignored)
├── cmd/              # Go 程序入口
├── internal/         # 后端私有代码
│   ├── config/       # 配置管理
│   ├── db/           # 数据库模型与操作
│   ├── handler/      # HTTP 路由处理
│   ├── media/        # 媒体扫描与转码核心
│   └── server/       # HTTP Server 生命周期
├── web/              # 前端 Vue 3 项目根目录
│   ├── dist/         # 前端构建产物 (嵌入到 Go)
│   ├── src/
│   │   ├── components/ # UI 组件 (Player, Sidebar...)
│   │   ├── services/   # API 请求封装 (Axios)
│   │   ├── stores/     # Pinia 状态管理
│   │   └── types/      # TypeScript 类型定义
│   └── vite.config.ts  # Vite 配置
└── scripts/          # 构建与开发辅助脚本
```

## ⚠️ 开发注意事项

1.  **前后端联调**: 
    - 开发模式下，Vite 配置了代理 (`/api` -> `localhost:8099`)。
    - 确保后端服务已启动，否则前端 API 请求会失败。
2.  **静态资源**:
    - 前端图片/字体请放在 `web/src/assets` 或 `web/public`。
    - Go 代码中通过 `embed` 包将 `web/dist` 打包进二进制。
3.  **代码规范**:
    - 后端：提交前请运行 `go fmt ./...` 和 `go vet ./...`。
    - 前端：TypeScript 类型检查通过 (`pnpm build` 会自动执行 `vue-tsc`)。
