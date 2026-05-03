#requires -Version 5.1
param(
  [string[]]$Platforms = @('windows'),
  [string[]]$Architectures = @('x64'),
  [switch]$SkipTests = $false,
  [switch]$SkipLint = $false
)

$ErrorActionPreference = 'Stop'

# 设置 UTF-8 编码以减少乱码
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$logFile = Join-Path $PSScriptRoot 'build.log'

function Write-Log {
  param(
    [Parameter(Mandatory = $true)][string]$Message,
    [ValidateSet('INFO', 'WARN', 'ERROR', 'SUCCESS')][string]$Level = 'INFO'
  )
  $ts = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss.fff')
  $line = "[$ts][$Level] $Message"
  $colorMap = @{ 'INFO' = 'White'; 'WARN' = 'Yellow'; 'ERROR' = 'Red'; 'SUCCESS' = 'Green' }
  Write-Host $line -ForegroundColor $colorMap[$Level]
  try { $line | Out-File -FilePath $logFile -Append -Encoding utf8 } catch {}
}

function Invoke-Step {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][scriptblock]$Action
  )
  Write-Log $Name 'INFO'
  try {
    & $Action
    Write-Log "$Name done." 'SUCCESS'
  }
  catch {
    Write-Log "$Name failed: $($_.Exception.Message)" 'ERROR'
    throw
  }
}

# 检查依赖
function Test-Dependency {
  param([string]$Name, [string]$Command)
  if (-not (Get-Command $Command -ErrorAction SilentlyContinue)) {
    Write-Log "$Name not found. Please install $Name." 'ERROR'
    exit 1
  }
}

Test-Dependency 'Go' 'go'

Invoke-Step 'Build Frontend' {
  if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    Write-Log 'pnpm not found. Installing pnpm via corepack...' 'WARN'
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
      if ($LASTEXITCODE -ne 0) { throw "pnpm install failed. exitCode=$LASTEXITCODE" }
    }
    Write-Log 'Building frontend...' 'INFO'
    pnpm run build
    if ($LASTEXITCODE -ne 0) { throw "pnpm run build failed. exitCode=$LASTEXITCODE" }
  }
  finally {
    Pop-Location
  }
}

if (-not $SkipTests) {
  Invoke-Step 'Run Go Tests' {
    Push-Location $root
    try {
      Remove-Item Env:GOOS -ErrorAction SilentlyContinue
      Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
      Remove-Item Env:GOARM -ErrorAction SilentlyContinue
      Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
      & go test -v ./...
      if ($LASTEXITCODE -ne 0) { throw "go test failed. exitCode=$LASTEXITCODE" }
    }
    finally {
      Pop-Location
    }
  }
}

if (-not $SkipLint) {
  Invoke-Step 'Run Go Vet' {
    Push-Location $root
    try {
      & go vet ./...
      if ($LASTEXITCODE -ne 0) { throw "go vet failed. exitCode=$LASTEXITCODE" }
    }
    finally {
      Pop-Location
    }
  }

  # 检查 golangci-lint 是否可用
  if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    Invoke-Step 'Run golangci-lint' {
      Push-Location $root
      try {
        & golangci-lint run ./...
        if ($LASTEXITCODE -ne 0) { throw "golangci-lint failed. exitCode=$LASTEXITCODE" }
      }
      finally {
        Pop-Location
      }
    }
  } else {
    Write-Log 'golangci-lint not found, skipping lint check. Install from https://golangci-lint.run/' 'WARN'
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
    if ($LASTEXITCODE -ne 0) { throw "go build failed. exitCode=$LASTEXITCODE" }
    Write-Log "Built: $OutPath" 'SUCCESS'
  }
  finally {
    Pop-Location
  }
}

function Write-Checksum {
  param([string]$FilePath, [string]$ChecksumPath)
  $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $FilePath
  $line = ($hash.Hash + "  " + [System.IO.Path]::GetFileName($FilePath))
  New-Dir ([System.IO.Path]::GetDirectoryName($ChecksumPath))
  $line | Out-File -FilePath $ChecksumPath -Encoding ascii
  Write-Log "Checksum: $ChecksumPath" 'INFO'
}

function ShouldBuild {
  param([string]$Platform, [string]$ArchOrVariant)

  $norm = @{ 'x64' = 'amd64'; 'x86' = '386' }
  $target = $ArchOrVariant.ToLower()
  if ($norm.ContainsKey($target)) { $target = $norm[$target] }

  $pMatch = $false
  foreach ($p in $Platforms) {
    if ($p.ToLower() -eq $Platform.ToLower()) { $pMatch = $true; break }
  }
  if (-not $pMatch) { return $false }

  foreach ($a in $Architectures) {
    $inputA = $a.ToLower()
    if ($norm.ContainsKey($inputA)) { $inputA = $norm[$inputA] }
    if ($inputA -eq $target) { return $true }
  }
  return $false
}

Invoke-Step 'Cross Build Artifacts' {
  $binRoot = Join-Path $root 'bin'
  $chkRoot = Join-Path $root 'checksums'

  $buildConfigs = @(
    @{ Platform = 'linux';   Arch = 'amd64'; OutName = 'msp-linux-amd64' },
    @{ Platform = 'linux';   Arch = 'arm64'; OutName = 'msp-linux-arm64' },
    @{ Platform = 'linux';   Arch = 'arm';   GOARM = '7'; OutName = 'msp-linux-armv7' },
    @{ Platform = 'darwin';  Arch = 'amd64'; OutName = 'msp-darwin-amd64' },
    @{ Platform = 'darwin';  Arch = 'arm64'; OutName = 'msp-darwin-arm64' },
    @{ Platform = 'windows'; Arch = 'amd64'; OutName = 'msp-windows-amd64.exe' },
    @{ Platform = 'windows'; Arch = '386';   OutName = 'msp-windows-386.exe' }
  )

  foreach ($cfg in $buildConfigs) {
    $platform = $cfg.Platform
    $arch = $cfg.Arch
    $outName = $cfg.OutName
    $goarm = $cfg.GOARM

    $shouldBuild = $false
    if ($platform -eq 'linux' -and $arch -eq 'amd64') {
      $shouldBuild = (ShouldBuild 'linux' 'amd64') -or (ShouldBuild 'linux' 'x64')
    } elseif ($platform -eq 'linux' -and $arch -eq 'arm64') {
      $shouldBuild = (ShouldBuild 'linux' 'arm64')
    } elseif ($platform -eq 'linux' -and $arch -eq 'arm') {
      $shouldBuild = (ShouldBuild 'arm' 'v7')
    } elseif ($platform -eq 'darwin' -and $arch -eq 'amd64') {
      $shouldBuild = (ShouldBuild 'macos' 'amd64') -or (ShouldBuild 'macos' 'x64')
    } elseif ($platform -eq 'darwin' -and $arch -eq 'arm64') {
      $shouldBuild = (ShouldBuild 'macos' 'arm64')
    } elseif ($platform -eq 'windows' -and $arch -eq 'amd64') {
      $shouldBuild = (ShouldBuild 'windows' 'amd64') -or (ShouldBuild 'windows' 'x64')
    } elseif ($platform -eq 'windows' -and $arch -eq '386') {
      $shouldBuild = (ShouldBuild 'windows' '386') -or (ShouldBuild 'windows' 'x86')
    }

    if ($shouldBuild) {
      $outPath = Join-Path $binRoot "$platform/$arch/$outName"
      Build-Go $platform $arch $outPath $goarm
      $chkPath = Join-Path $chkRoot "$outName.sha256"
      Write-Checksum $outPath $chkPath
    }
  }
}

Write-Log 'Build completed.' 'SUCCESS'
