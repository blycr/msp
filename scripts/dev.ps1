#requires -Version 5.1
param(
  [int]$BackendPort = 8099
)

$ErrorActionPreference = 'Stop'

# 设置 UTF-8 编码以减少乱码
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$backendExe = Join-Path $root 'bin/dev/msp-dev.exe'
$devDir = Split-Path $backendExe -Parent
$devConfig = Join-Path $devDir 'config.json'
$devConfigExample = Join-Path $root 'config.example.json'

function Write-Log {
  param(
    [Parameter(Mandatory = $true)][string]$Message,
    [ValidateSet('INFO', 'WARN', 'ERROR', 'SUCCESS')][string]$Level = 'INFO'
  )
  $colorMap = @{ 'INFO' = 'White'; 'WARN' = 'Yellow'; 'ERROR' = 'Red'; 'SUCCESS' = 'Green' }
  Write-Host "[dev] $Message" -ForegroundColor $colorMap[$Level]
}

function Build-Backend {
  Write-Log 'Building backend...'
  Push-Location $root
  try {
    & go build -o $backendExe ./cmd/msp
    if ($LASTEXITCODE -ne 0) { throw "go build failed. exitCode=$LASTEXITCODE" }
    Write-Log 'Backend built successfully.' 'SUCCESS'
  }
  finally {
    Pop-Location
  }
}

$script:backendProc = $null
$script:frontendProc = $null

function Stop-Backend {
  if ($script:backendProc -and -not $script:backendProc.HasExited) {
    Write-Log "Stopping backend (pid=$($script:backendProc.Id))..."
    try {
      # 首先尝试优雅关闭 (发送 Ctrl+C 信号)
      # Windows 上没有直接的 Ctrl+C 信号，所以我们使用 Kill()
      $script:backendProc.Kill()
      if (-not $script:backendProc.WaitForExit(5000)) {
        Write-Log 'Backend did not exit gracefully, forcing termination...' 'WARN'
        Stop-Process -Id $script:backendProc.Id -Force -ErrorAction SilentlyContinue
      }
    }
    catch {
      Write-Log "Error stopping backend: $($_.Exception.Message)" 'WARN'
    }
    $script:backendProc = $null
  }
}

function Stop-Frontend {
  if ($script:frontendProc -and -not $script:frontendProc.HasExited) {
    Write-Log "Stopping frontend (pid=$($script:frontendProc.Id))..."
    try {
      Stop-Process -Id $script:frontendProc.Id -Force -ErrorAction SilentlyContinue
    }
    catch {
      Write-Log "Error stopping frontend: $($_.Exception.Message)" 'WARN'
    }
    $script:frontendProc = $null
  }
}

function Initialize-DevConfig {
  if (-not (Test-Path $devDir)) {
    New-Item -ItemType Directory -Force -Path $devDir | Out-Null
  }
  if (-not (Test-Path $devConfig)) {
    if (Test-Path $devConfigExample) {
      Copy-Item -LiteralPath $devConfigExample -Destination $devConfig -Force
    }
    else {
      '{}' | Out-File -FilePath $devConfig -Encoding utf8
    }
  }

  $cfg = try { Get-Content -LiteralPath $devConfig -Raw | ConvertFrom-Json -ErrorAction Stop } catch { [pscustomobject]@{} }

  # Ensure necessary fields exist
  $changed = $false
  if ($null -eq $cfg.port -or $cfg.port -ne $BackendPort) { $cfg | Add-Member -Name port -Value $BackendPort -MemberType NoteProperty -Force; $changed = $true }
  if ($null -eq $cfg.blacklist) { $cfg | Add-Member -Name blacklist -Value ([pscustomobject]@{}) -MemberType NoteProperty -Force; $changed = $true }
  if ($null -eq $cfg.blacklist.extensions) { $cfg.blacklist | Add-Member -Name extensions -Value @() -MemberType NoteProperty -Force; $changed = $true }
  if ($null -eq $cfg.blacklist.filenames) { $cfg.blacklist | Add-Member -Name filenames -Value @() -MemberType NoteProperty -Force; $changed = $true }
  if ($null -eq $cfg.blacklist.folders) { $cfg.blacklist | Add-Member -Name folders -Value @() -MemberType NoteProperty -Force; $changed = $true }
  if ($null -eq $cfg.blacklist.sizeRule) { $cfg.blacklist | Add-Member -Name sizeRule -Value '' -MemberType NoteProperty -Force; $changed = $true }

  if ($changed) {
    $cfg | ConvertTo-Json -Depth 20 | Out-File -FilePath $devConfig -Encoding utf8
    Write-Log "Updated dev config with port $BackendPort"
  }
}

function Start-Backend {
  Stop-Backend
  Initialize-DevConfig
  Write-Log 'Starting backend...'
  $psi = New-Object System.Diagnostics.ProcessStartInfo
  $psi.FileName = $backendExe
  $psi.WorkingDirectory = $devDir
  $psi.UseShellExecute = $false
  $psi.EnvironmentVariables['MSP_NO_AUTO_OPEN'] = '1'
  # 允许进程在窗口关闭时继续运行
  $script:backendProc = [System.Diagnostics.Process]::Start($psi)
  Write-Log "Backend started (pid=$($script:backendProc.Id))" 'SUCCESS'
  # 等待后端启动
  Start-Sleep -Seconds 2
}

function Start-Frontend {
  Stop-Frontend
  # Check if pnpm is installed
  if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    Write-Log 'pnpm not found. Enabling corepack...' 'WARN'
    corepack enable
    if ($LASTEXITCODE -ne 0) {
      throw 'pnpm is not installed and corepack enable failed. Please install pnpm: npm install -g pnpm'
    }
  }

  Write-Log 'Starting frontend (Vite dev server)...'
  $webRoot = Join-Path $root 'web'
  Push-Location $webRoot
  try {
    if (-not (Test-Path 'node_modules')) {
      Write-Log 'Installing pnpm dependencies...'
      pnpm install
      if ($LASTEXITCODE -ne 0) { throw "pnpm install failed. exitCode=$LASTEXITCODE" }
    }
    $cmd = "`$env:MSP_DEV_BACKEND='http://127.0.0.1:$BackendPort'; pnpm run dev"
    $psExe = (Get-Process -Id $PID).Path
    $script:frontendProc = Start-Process -FilePath $psExe -ArgumentList '-NoLogo', '-NoProfile', '-Command', $cmd -WorkingDirectory $webRoot -PassThru
    Write-Log "Frontend started (pid=$($script:frontendProc.Id))" 'SUCCESS'
  }
  finally {
    Pop-Location
  }
}

# 注册退出处理
$exitHandler = {
  Write-Log 'Shutting down development server...'
  Stop-Frontend
  Stop-Backend
  Write-Log 'Cleanup completed.' 'SUCCESS'
}

# 注册事件处理器
$null = Register-EngineEvent -SourceIdentifier PowerShell.Exiting -Action $exitHandler

Write-Log "Root: $root"
Write-Log "Backend dev port: $BackendPort"

# 初始构建和启动
Build-Backend
Start-Backend
Start-Frontend

Write-Log 'Development server is running. Press Ctrl+C or Q to stop.'
Write-Log 'Press R to manually rebuild backend.'

# 文件系统监视器
$fsw = New-Object System.IO.FileSystemWatcher $root, '*.go'
$fsw.IncludeSubdirectories = $true
$fsw.EnableRaisingEvents = $true
$fsw.NotifyFilter = [System.IO.NotifyFilters]::LastWrite -bor [System.IO.NotifyFilters]::FileName

# 防抖定时器
$script:lastChange = [DateTime]::MinValue
$script:rebuildPending = $false

try {
  while ($true) {
    # 检查按键
    if ([Console]::KeyAvailable) {
      $key = [Console]::ReadKey($true)
      if ($key.Key -eq 'Q' -or $key.Key -eq 'Escape') {
        Write-Log 'Stop key pressed, shutting down...'
        break
      }
      elseif ($key.Key -eq 'R') {
        Write-Log 'Manual rebuild triggered...'
        Build-Backend
        Start-Backend
      }
    }

    # 检查文件变化 (带防抖)
    $change = $fsw.WaitForChanged(
      [System.IO.WatcherChangeTypes]::Changed -bor
      [System.IO.WatcherChangeTypes]::Created -bor
      [System.IO.WatcherChangeTypes]::Deleted -bor
      [System.IO.WatcherChangeTypes]::Renamed,
      500
    )

    if (-not $change.TimedOut) {
      $now = [DateTime]::Now
      # 防抖: 1秒内不重复构建
      if (($now - $script:lastChange).TotalSeconds -gt 1) {
        $script:lastChange = $now
        Write-Log "Change detected in $($change.Name). Rebuilding backend..."
        Build-Backend
        Start-Backend
      }
    }
  }
}
finally {
  $fsw.EnableRaisingEvents = $false
  $fsw.Dispose()
  & $exitHandler
}
