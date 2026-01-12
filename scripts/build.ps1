param(
  [string[]]$Platforms = @('windows'),
  [string[]]$Architectures = @('x64')
)

$ErrorActionPreference = 'Stop'

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$logFile = Join-Path $PSScriptRoot 'build.log'

function Write-Log {
  param(
    [Parameter(Mandatory=$true)][string]$Message,
    [ValidateSet('INFO','WARN','ERROR')][string]$Level = 'INFO'
  )
  $ts = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss.fff')
  $line = "[$ts][$Level] $Message"
  Write-Host $line
  try { $line | Out-File -FilePath $logFile -Append -Encoding utf8 } catch {}
}

function Invoke-Step {
  param(
    [Parameter(Mandatory=$true)][string]$Name,
    [Parameter(Mandatory=$true)][scriptblock]$Action
  )
  Write-Log $Name 'INFO'
  try {
    & $Action
    Write-Log ($Name + ' done.') 'INFO'
  } catch {
    Write-Log ($Name + ' failed: ' + $_.Exception.Message) 'ERROR'
    throw
  }
}


Invoke-Step 'Build Frontend' {
  # Check if pnpm is installed
  if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    Write-Log 'pnpm not found. Installing pnpm via corepack...' 'INFO'
    corepack enable
    if ($LASTEXITCODE -ne 0) {
      throw "pnpm is not installed and corepack enable failed. Please install pnpm: npm install -g pnpm"
    }
  }
  
  Push-Location (Join-Path $root 'web')
  try {
    if (-not (Test-Path 'node_modules')) {
        Write-Log 'Installing pnpm dependencies...' 'INFO'
        pnpm install
        if ($LASTEXITCODE -ne 0) { throw ("pnpm install failed. exitCode=" + $LASTEXITCODE) }
    }
    Write-Log 'Building frontend...' 'INFO'
    pnpm run build
    if ($LASTEXITCODE -ne 0) { throw ("pnpm run build failed. exitCode=" + $LASTEXITCODE) }
  } finally {
    Pop-Location
  }
}

Invoke-Step 'go test ./...' {
  Push-Location $root
  try {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:GOARM -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    & go env -u GOOS
    & go env -u GOARCH
    & go env -u GOARM
    & go env -u CGO_ENABLED
    & go env -w GOOS=windows
    & go env -w GOARCH=amd64
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw ("go test failed. exitCode=" + $LASTEXITCODE) }
  } finally {
    Pop-Location
  }
}


function New-Dir {
  param([string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
  }
}

function Build-Go {
  param([string]$GOOS, [string]$GOARCH, [string]$OutPath, [string]$GOARM = $null)
  Push-Location $root
  try {
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    $env:CGO_ENABLED = "0"
    if ($GOARM) { $env:GOARM = $GOARM } else { Remove-Item Env:GOARM -ErrorAction SilentlyContinue }
    New-Dir ([System.IO.Path]::GetDirectoryName($OutPath))
    & go build -trimpath -ldflags="-s -w" -o $OutPath ./cmd/msp
    if ($LASTEXITCODE -ne 0) { throw ("go build failed. exitCode=" + $LASTEXITCODE) }
    Write-Log ("Built: " + $OutPath) 'INFO'
  } finally {
    Pop-Location
  }
}

function Write-Checksum {
  param([string]$FilePath, [string]$ChecksumPath)
  $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $FilePath
  $line = ($hash.Hash + "  " + [System.IO.Path]::GetFileName($FilePath))
  New-Dir ([System.IO.Path]::GetDirectoryName($ChecksumPath))
  $line | Out-File -FilePath $ChecksumPath -Encoding ascii
  Write-Log ("Checksum: " + $ChecksumPath) 'INFO'
}

function Write-DebugCopy {
  param([string]$FilePath, [string]$DebugPath)
  New-Dir ([System.IO.Path]::GetDirectoryName($DebugPath))
  Copy-Item -LiteralPath $FilePath -Destination $DebugPath -Force
  Write-Log ("Debug copy: " + $DebugPath) 'INFO'
}

function ShouldBuild {
  param([string]$Platform, [string]$ArchOrVariant)
  $pMatch = $false
  foreach ($p in $Platforms) {
    if ($p.ToLower() -eq $Platform.ToLower()) { $pMatch = $true; break }
  }
  if (-not $pMatch) { return $false }
  foreach ($a in $Architectures) {
    if ($a.ToLower() -eq $ArchOrVariant.ToLower()) { return $true }
  }
  return $false
}

Invoke-Step 'Cross Build Artifacts' {
  $binRoot = Join-Path $root 'bin'
  $chkRoot = Join-Path $root 'checksums'
  $dbgRoot = Join-Path $root 'debug'

  if (ShouldBuild 'linux' 'amd64') {
    Build-Go 'linux' 'amd64' (Join-Path $binRoot 'linux/amd64/msp-linux-amd64')
    Write-Checksum (Join-Path $binRoot 'linux/amd64/msp-linux-amd64') (Join-Path $chkRoot 'msp-linux-amd64.sha256')
    Write-DebugCopy (Join-Path $binRoot 'linux/amd64/msp-linux-amd64') (Join-Path $dbgRoot 'linux/amd64/msp-linux-amd64.debug')
  }

  if (ShouldBuild 'linux' 'arm64') {
    Build-Go 'linux' 'arm64' (Join-Path $binRoot 'linux/arm64/msp-linux-arm64')
    Write-Checksum (Join-Path $binRoot 'linux/arm64/msp-linux-arm64') (Join-Path $chkRoot 'msp-linux-arm64.sha256')
    Write-DebugCopy (Join-Path $binRoot 'linux/arm64/msp-linux-arm64') (Join-Path $dbgRoot 'linux/arm64/msp-linux-arm64.debug')
  }

  if (ShouldBuild 'arm' 'v7') {
    Build-Go 'linux' 'arm' (Join-Path $binRoot 'arm/v7/msp-arm-v7') '7'
    Write-Checksum (Join-Path $binRoot 'arm/v7/msp-arm-v7') (Join-Path $chkRoot 'msp-arm-v7.sha256')
    Write-DebugCopy (Join-Path $binRoot 'arm/v7/msp-arm-v7') (Join-Path $dbgRoot 'arm/v7/msp-arm-v7.debug')
  }

  if (ShouldBuild 'arm' 'v8') {
    Build-Go 'linux' 'arm64' (Join-Path $binRoot 'arm/v8/msp-arm-v8')
    Write-Checksum (Join-Path $binRoot 'arm/v8/msp-arm-v8') (Join-Path $chkRoot 'msp-arm-v8.sha256')
    Write-DebugCopy (Join-Path $binRoot 'arm/v8/msp-arm-v8') (Join-Path $dbgRoot 'arm/v8/msp-arm-v8.debug')
  }

  if (ShouldBuild 'macos' 'amd64') {
    Build-Go 'darwin' 'amd64' (Join-Path $binRoot 'macos/msp-macos-amd64')
    Write-Checksum (Join-Path $binRoot 'macos/msp-macos-amd64') (Join-Path $chkRoot 'msp-macos-amd64.sha256')
    Write-DebugCopy (Join-Path $binRoot 'macos/msp-macos-amd64') (Join-Path $dbgRoot 'macos/msp-macos-amd64.debug')
  }

  if (ShouldBuild 'macos' 'arm64') {
    Build-Go 'darwin' 'arm64' (Join-Path $binRoot 'macos/msp-macos-arm64')
    Write-Checksum (Join-Path $binRoot 'macos/msp-macos-arm64') (Join-Path $chkRoot 'msp-macos-arm64.sha256')
    Write-DebugCopy (Join-Path $binRoot 'macos/msp-macos-arm64') (Join-Path $dbgRoot 'macos/msp-macos-arm64.debug')
  }

  if (ShouldBuild 'windows' 'x64') {
    Build-Go 'windows' 'amd64' (Join-Path $binRoot 'windows/x64/msp-windows-amd64.exe')
    Write-Checksum (Join-Path $binRoot 'windows/x64/msp-windows-amd64.exe') (Join-Path $chkRoot 'msp-windows-amd64.sha256')
    Write-DebugCopy (Join-Path $binRoot 'windows/x64/msp-windows-amd64.exe') (Join-Path $dbgRoot 'windows/x64/msp-windows-amd64.debug')
  }

  if (ShouldBuild 'windows' 'x86') {
    Build-Go 'windows' '386' (Join-Path $binRoot 'windows/x86/msp-windows-386.exe')
    Write-Checksum (Join-Path $binRoot 'windows/x86/msp-windows-386.exe') (Join-Path $chkRoot 'msp-windows-386.sha256')
    Write-DebugCopy (Join-Path $binRoot 'windows/x86/msp-windows-386.exe') (Join-Path $dbgRoot 'windows/x86/msp-windows-386.debug')
  }
}

Write-Log 'Build completed.' 'INFO'

