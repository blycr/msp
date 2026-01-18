param(
  [int]$BackendPort = 8099
)

$ErrorActionPreference = 'Stop'

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

# Detect V2
$isV2 = Test-Path (Join-Path $root 'web/vite.config.ts')
$suffix = if ($isV2) { "-v2" } else { "" }
if ($isV2) { Write-Host "[dev] Detected V2 Frontend. Using suffix: $suffix" }

$backendExe = Join-Path $root "bin/dev/msp$suffix-dev.exe"
$devDir = Split-Path $backendExe -Parent
$devConfig = Join-Path $devDir 'config.json'
$devConfigExample = Join-Path $root 'config.example.json'

function Build-Backend {
  Write-Host "[dev] Building backend..."
  Push-Location $root
  try {
    & go build -o $backendExe ./cmd/msp
    if ($LASTEXITCODE -ne 0) { throw "go build failed. exitCode=$LASTEXITCODE" }
  }
  finally {
    Pop-Location
  }
}

$script:backendProc = $null
$script:frontendProc = $null

function Stop-Backend {
  if ($script:backendProc -and -not $script:backendProc.HasExited) {
    Write-Host "[dev] Stopping backend (pid=$($script:backendProc.Id))..."
    try {
      $script:backendProc.Kill()
      $script:backendProc.WaitForExit()
    }
    catch {}
  }
}

function Stop-Frontend {
  if ($script:frontendProc -and -not $script:frontendProc.HasExited) {
    Write-Host "[dev] Stopping frontend (pid=$($script:frontendProc.Id))..."
    try {
      $script:frontendProc.Kill()
      $script:frontendProc.WaitForExit()
    }
    catch {}
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
  if ($null -eq $cfg.blacklist.sizeRule) { $cfg.blacklist | Add-Member -Name sizeRule -Value "" -MemberType NoteProperty -Force; $changed = $true }

  if ($changed) {
    $cfg | ConvertTo-Json -Depth 20 | Out-File -FilePath $devConfig -Encoding utf8
  }
}

function Start-Backend {
  Stop-Backend
  Initialize-DevConfig
  Write-Host "[dev] Starting backend..."
  $psi = New-Object System.Diagnostics.ProcessStartInfo
  $psi.FileName = $backendExe
  $psi.WorkingDirectory = $devDir
  $psi.UseShellExecute = $false
  $psi.EnvironmentVariables["MSP_NO_AUTO_OPEN"] = "1"
  $script:backendProc = [System.Diagnostics.Process]::Start($psi)
}

function Start-Frontend {
  Stop-Frontend
  # Check if pnpm is installed
  if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    Write-Host "[dev] pnpm not found. Enabling corepack..." 
    corepack enable
    if ($LASTEXITCODE -ne 0) {
      throw "pnpm is not installed and corepack enable failed. Please install pnpm: npm install -g pnpm"
    }
  }
  
  Write-Host "[dev] Starting frontend (Vite dev server)..."
  $webRoot = Join-Path $root 'web'
  Push-Location $webRoot
  try {
    if (-not (Test-Path 'node_modules')) {
      Write-Host "[dev] Installing pnpm dependencies..."
      pnpm install
      if ($LASTEXITCODE -ne 0) { throw "pnpm install failed. exitCode=$LASTEXITCODE" }
    }
    $cmd = "`$env:MSP_DEV_BACKEND='http://127.0.0.1:$BackendPort'; pnpm run dev"
    $psExe = (Get-Process -Id $PID).Path
    $script:frontendProc = Start-Process -FilePath $psExe -ArgumentList "-NoLogo", "-NoProfile", "-Command", $cmd -WorkingDirectory $webRoot -PassThru
  }
  finally {
    Pop-Location
  }
}

Write-Host "[dev] Root: $root"
Write-Host "[dev] Backend dev port: $BackendPort"

Build-Backend
Start-Backend
Start-Frontend

Write-Host "[dev] Watching Go files for changes... (press any key to stop)"
$fsw = New-Object System.IO.FileSystemWatcher $root, "*.go"
$fsw.IncludeSubdirectories = $true
$fsw.EnableRaisingEvents = $true

try {
  while ($true) {
    if ([Console]::KeyAvailable) {
      [void][Console]::ReadKey($true)
      Write-Host "[dev] Stop key pressed, shutting down..."
      break
    }
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
    Write-Host "[dev] Change detected in $($change.Name). Rebuilding backend..."
    Build-Backend
    Start-Backend
  }
}
finally {
  $fsw.EnableRaisingEvents = $false
  $fsw.Dispose()
  Stop-Backend
  Stop-Frontend
}
