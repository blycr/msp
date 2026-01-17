# 构建与开发脚本说明

本文档详细说明了 `build.ps1` 和 `dev.ps1` 两个 PowerShell 脚本的启动流程和使用方法。

---

## 1. `build.ps1` - 生产构建脚本

### 参数初始化
```powershell
param(
  [string[]]$Platforms = @('windows'),
  [string[]]$Architectures = @('x64')
)
```
- 默认构建 Windows x64 平台
- 支持跨平台构建（Linux、macOS、Windows）

### 启动流程

#### 步骤 1: 构建前端
```powershell
Invoke-Step 'Build Frontend' {
  # 检查并安装 pnpm
  if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    corepack enable
  }
  
  # 进入 web 目录
  Push-Location (Join-Path $root 'web')
  
  # 安装依赖（如果需要）
  if (-not (Test-Path 'node_modules')) {
    pnpm install
  }
  
  # 构建前端
  pnpm run build
}
```

#### 步骤 2: 运行测试
```powershell
Invoke-Step 'go test ./...' {
  # 清理环境变量
  Remove-Item Env:GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
  
  # 运行所有测试
  & go test ./...
}
```

#### 步骤 3: 跨平台构建二进制文件
```powershell
Invoke-Step 'Cross Build Artifacts' {
  # 根据参数构建不同平台的二进制文件
  # Linux amd64, arm64
  # ARM v7, v8
  # macOS amd64, arm64
  # Windows x64, x86
  
  # 每个平台执行：
  Build-Go 'GOOS' 'GOARCH' '输出路径'
  Write-Checksum '文件路径' '校验和路径'
  Write-DebugCopy '文件路径' '调试副本路径'
}
```

### 构建函数说明

#### Build-Go 函数
```powershell
function Build-Go {
  param([string]$GOOS, [string]$GOARCH, [string]$OutPath, [string]$GOARM = $null)
  $env:GOOS = $GOOS
  $env:GOARCH = $GOARCH
  $env:CGO_ENABLED = "0"
  
  # 构建优化后的二进制文件
  & go build -trimpath -ldflags="-s -w" -o $OutPath ./cmd/msp
}
```
- `-trimpath`: 移除文件系统路径
- `-ldflags="-s -w"`: 去除调试信息和符号表，减小文件大小

#### 输出目录结构
```
bin/
├── linux/amd64/msp-linux-amd64
├── linux/arm64/msp-linux-arm64
├── arm/v7/msp-arm-v7
├── arm/v8/msp-arm-v8
├── macos/msp-macos-amd64
├── macos/msp-macos-arm64
├── windows/x64/msp-windows-amd64.exe
└── windows/x86/msp-windows-386.exe

checksums/
├── msp-linux-amd64.sha256
├── msp-linux-arm64.sha256
...

debug/
├── linux/amd64/msp-linux-amd64.debug
...
```

### 使用示例

```powershell
# 构建默认平台（Windows x64）
.\scripts\build.ps1

# 构建所有平台
.\scripts\build.ps1 -Platforms @('linux', 'macos', 'windows', 'arm') -Architectures @('x64', 'arm64', 'x86', 'v7', 'v8')

# 仅构建 Linux 平台
.\scripts\build.ps1 -Platforms @('linux') -Architectures @('x64', 'arm64')
```

---

## 2. `dev.ps1` - 开发环境脚本

### 参数初始化
```powershell
param(
  [int]$BackendPort = 8099  # 默认后端端口
)
```

### 启动流程

#### 步骤 1: 构建后端
```powershell
function Build-Backend {
  Push-Location $root
  try {
    # 构建开发版本二进制文件
    & go build -o $backendExe ./cmd/msp
  }
  finally {
    Pop-Location
  }
}
```
- 输出到 `bin/dev/msp-dev.exe`

#### 步骤 2: 初始化开发配置
```powershell
function Initialize-DevConfig {
  # 创建开发目录
  if (-not (Test-Path $devDir)) {
    New-Item -ItemType Directory -Force -Path $devDir
  }
  
  # 复制示例配置或创建空配置
  if (-not (Test-Path $devConfig)) {
    if (Test-Path $devConfigExample) {
      Copy-Item -LiteralPath $devConfigExample -Destination $devConfig
    }
    else {
      '{}' | Out-File -FilePath $devConfig -Encoding utf8
    }
  }
  
  # 确保必要字段存在
  # - port: 8099
  # - blacklist.extensions: []
  # - blacklist.filenames: []
  # - blacklist.folders: []
  # - blacklist.sizeRule: ""
}
```

#### 步骤 3: 启动后端服务
```powershell
function Start-Backend {
  Stop-Backend  # 先停止旧进程
  Initialize-DevConfig
  
  $psi = New-Object System.Diagnostics.ProcessStartInfo
  $psi.FileName = $backendExe
  $psi.WorkingDirectory = $devDir
  $psi.UseShellExecute = $false
  $psi.EnvironmentVariables["MSP_NO_AUTO_OPEN"] = "1"  # 禁用自动打开浏览器
  
  $script:backendProc = [System.Diagnostics.Process]::Start($psi)
}
```

#### 步骤 4: 启动前端开发服务器
```powershell
function Start-Frontend {
  Stop-Frontend  # 先停止旧进程
  
  # 检查并安装 pnpm
  if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    corepack enable
  }
  
  # 进入 web 目录
  Push-Location $webRoot
  
  # 安装依赖（如果需要）
  if (-not (Test-Path 'node_modules')) {
    pnpm install
  }
  
  # 启动 Vite 开发服务器
  $cmd = "`$env:MSP_DEV_BACKEND='http://127.0.0.1:$BackendPort'; pnpm run dev"
  $script:frontendProc = Start-Process -FilePath $psExe -ArgumentList "-NoLogo", "-NoProfile", "-Command", $cmd -WorkingDirectory $webRoot -PassThru
}
```
- 设置环境变量 `MSP_DEV_BACKEND` 指向后端地址
- 启动 Vite 热重载开发服务器

#### 步骤 5: 文件监听与自动重启
```powershell
Write-Host "[dev] Watching Go files for changes... (press any key to stop)"

# 创建文件系统监视器
$fsw = New-Object System.IO.FileSystemWatcher $root, "*.go"
$fsw.IncludeSubdirectories = $true
$fsw.EnableRaisingEvents = $true

try {
  while ($true) {
    # 检测按键
    if ([Console]::KeyAvailable) {
      [void][Console]::ReadKey($true)
      Write-Host "[dev] Stop key pressed, shutting down..."
      break
    }
    
    # 等待文件变化（500ms 超时）
    $change = $fsw.WaitForChanged(
      [System.IO.WatcherChangeTypes]::Changed -bor
      [System.IO.WatcherChangeTypes]::Created -bor
      [System.IO.WatcherChangeTypes]::Deleted -bor
      [System.IO.WatcherChangeTypes]::Renamed,
      500
    )
    
    if ($change.TimedOut) {
      continue
    }
    
    # 检测到变化，重新构建并重启后端
    Write-Host "[dev] Change detected in $($change.Name). Rebuilding backend..."
    Build-Backend
    Start-Backend
  }
}
finally {
  # 清理资源
  $fsw.EnableRaisingEvents = $false
  $fsw.Dispose()
  Stop-Backend
  Stop-Frontend
}
```

### 开发环境特点

1. **热重载**: 监听 `.go` 文件变化，自动重新构建并重启后端
2. **前端热更新**: Vite 开发服务器提供前端热更新
3. **独立配置**: 使用 `bin/dev/config.json` 作为开发配置
4. **进程管理**: 自动管理前后端进程的生命周期
5. **优雅退出**: 按任意键或 Ctrl+C 可停止所有服务

### 使用示例

```powershell
# 使用默认端口（8099）启动开发环境
.\scripts\dev.ps1

# 使用自定义端口启动开发环境
.\scripts\dev.ps1 -BackendPort 3000
```

---

## 总结对比

| 特性 | build.ps1 | dev.ps1 |
|------|-----------|---------|
| 用途 | 生产构建 | 开发环境 |
| 前端 | 构建静态文件 | Vite 开发服务器 |
| 后端 | 跨平台编译 | 本地开发服务器 |
| 测试 | 运行测试 | 不运行测试 |
| 热重载 | 无 | Go 文件自动重启 |
| 配置 | 使用用户配置 | 独立开发配置 |
| 输出 | 多平台二进制文件 | 单一开发版本 |
| 监听 | 无 | 文件系统监听 |

---

## 工作流程图

### build.ps1 工作流程
```
开始
  ↓
检查 pnpm → 安装 pnpm（如需要）
  ↓
安装前端依赖（如需要）
  ↓
构建前端（pnpm run build）
  ↓
运行测试（go test ./...）
  ↓
根据参数构建各平台二进制文件
  ↓
生成校验和（SHA256）
  ↓
创建调试副本
  ↓
完成
```

### dev.ps1 工作流程
```
开始
  ↓
构建后端（go build）
  ↓
初始化开发配置
  ↓
启动后端服务（端口 8099）
  ↓
检查 pnpm → 安装 pnpm（如需要）
  ↓
安装前端依赖（如需要）
  ↓
启动前端开发服务器（Vite）
  ↓
监听 .go 文件变化
  ↓
检测到变化 → 重新构建后端 → 重启后端
  ↓
按任意键停止
  ↓
清理资源 → 完成
```

---

## 注意事项

1. **pnpm 安装**: 两个脚本都依赖 pnpm，如果未安装会尝试通过 corepack 自动启用
2. **端口冲突**: dev.ps1 默认使用 8099 端口，如需修改请使用 `-BackendPort` 参数
3. **文件监听**: dev.ps1 仅监听 `.go` 文件，前端文件修改由 Vite 自动处理
4. **构建优化**: build.ps1 使用 `-ldflags="-s -w"` 优化二进制文件大小
5. **跨平台编译**: build.ps1 支持多平台编译，但需要确保 Go 环境配置正确
