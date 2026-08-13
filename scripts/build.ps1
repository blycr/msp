#requires -Version 5.1
param(
  [Alias('P')][string]$Preset,
  [Alias('F')][string[]]$Platforms,
  [Alias('A')][string[]]$Architectures,
  [Alias('T')][switch]$SkipTests,
  [Alias('L')][switch]$SkipLint,
  [Alias('H')][switch]$Help,
  [Alias('I')][switch]$ListPresets
)

# 本机 Windows PowerShell 5.1 的 Get-FileHash 不可用（模块加载异常）。
# 运行在 5.1 下且存在 pwsh(PS 7+) 时，自动以 pwsh 重新执行并转发全部参数。
if ($PSVersionTable.PSVersion.Major -lt 7) {
  $pwshCmd = Get-Command pwsh -ErrorAction SilentlyContinue
  if ($pwshCmd) {
    & $pwshCmd.Source -NoProfile -ExecutionPolicy Bypass -File $PSCommandPath @PSBoundParameters
    exit $LASTEXITCODE
  }
  throw 'This script requires PowerShell 7+ (pwsh): Get-FileHash is unavailable in this Windows PowerShell 5.1 session.'
}

$ErrorActionPreference = 'Stop'

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$logFile = Join-Path $PSScriptRoot 'build.log'
$profilesFile = Join-Path $PSScriptRoot 'build-profiles.json'
$script:MspVersion = if ($env:MSP_VERSION) { $env:MSP_VERSION } else {
  $desc = git -C $root describe --tags --always --dirty 2>$null
  if ($desc) { $desc.Trim() } else { 'dev' }
}
$script:MspLdflags = "-s -w -X main.version=$($script:MspVersion)"

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

function New-Dir {
  param([string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
  }
}

function Load-Profiles {
  if (-not (Test-Path $profilesFile)) {
    Write-Log "Profiles config not found: $profilesFile" 'WARN'
    return $null
  }
  try {
    return Get-Content -LiteralPath $profilesFile -Raw -Encoding UTF8 | ConvertFrom-Json
  }
  catch {
    Write-Log "Failed to parse profiles config: $($_.Exception.Message)" 'ERROR'
    return $null
  }
}

function Split-CommaValues {
  param([string[]]$Values)
  $result = @()
  foreach ($v in $Values) {
    if ($v -match ',') {
      $result += ($v -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne '' })
    }
    else {
      $result += $v
    }
  }
  return $result
}

function Show-Help {
$lines = @'

  MSP Build Script - Production Build Tool

  Usage:
    .\scripts\build.ps1 [options]

  Preset Mode (recommended):
    .\scripts\build.ps1 -P <name>

  Available Presets:
    all        All platforms and architectures
    release    Release build (same as all)
    linux      Linux all architectures (amd64, arm64, armv7, loong64)
    macos      macOS all architectures (amd64, arm64)
    darwin     macOS all architectures (alias)
    windows    Windows all architectures (amd64, x86)
    arm        ARM all architectures (arm64, armv7)
    server     Server deploy (Linux amd64 + arm64)
    desktop    Desktop (Windows + macOS)
    quick      Quick build (skip tests and lint)

  Custom Build:
    .\scripts\build.ps1 -F linux,windows -A x64,arm64

  Parameters:
    -P <name>          Preset config (alias: -Preset)
    -F <list>          Target platforms, comma-separated (alias: -Platforms)
    -A <list>          Target architectures, comma-separated (alias: -Architectures)
    -T                 Skip Go tests (alias: -SkipTests)
    -L                 Skip code checks (alias: -SkipLint)
    -I                 List all available presets (alias: -ListPresets)
    -H                 Show this help (alias: -Help)

  Examples:
    .\scripts\build.ps1 -P all
    .\scripts\build.ps1 -P windows
    .\scripts\build.ps1 -P quick
    .\scripts\build.ps1 -P server -T
    .\scripts\build.ps1 -F linux -A x64

'@
  Write-Host $lines
}

function Show-Presets {
  $profiles = Load-Profiles
  if (-not $profiles) { return }

  Write-Host "`n  Available Presets:`n" -ForegroundColor Cyan
  Write-Host ("  {0,-12} {1,-36} {2}" -f 'Name', 'Description', 'Targets') -ForegroundColor Gray
  Write-Host ("  {0,-12} {1,-36} {2}" -f '----', '-----------', '-------') -ForegroundColor Gray

  foreach ($name in ($profiles.presets.PSObject.Properties.Name | Sort-Object)) {
    $p = $profiles.presets.$name
    $platforms = $p.platforms -join ','
    $archs = $p.architectures -join ','
    $desc = if ($p.description) { $p.description } else { '' }
    $flags = ''
    if ($p.skipTests) { $flags += ' [skipTests]' }
    if ($p.skipLint) { $flags += ' [skipLint]' }
    $target = "$platforms | $archs$flags"
    Write-Host ("  {0,-12} {1,-36} {2}" -f $name, $desc, $target)
  }
  Write-Host ''
}

function Resolve-Preset {
  param([string]$Name)

  $profiles = Load-Profiles
  if (-not $profiles) {
    Write-Log 'Failed to load profiles config, falling back to defaults' 'WARN'
    return
  }

  $lower = $Name.ToLower()
  if (-not $profiles.presets.PSObject.Properties[$lower]) {
    Write-Log "Unknown preset: $Name" 'ERROR'
    Write-Host "`n  Available presets:" -ForegroundColor Yellow
    foreach ($n in ($profiles.presets.PSObject.Properties.Name | Sort-Object)) {
      $d = $profiles.presets.$n.description
      if ($d) { Write-Host "    $n  - $d" } else { Write-Host "    $n" }
    }
    Write-Host ''
    exit 1
  }

  $p = $profiles.presets.$lower
  $script:Platforms = $p.platforms
  $script:Architectures = $p.architectures
  if ($p.skipTests -and -not $SkipTests) { $script:SkipTests = [switch]$true }
  if ($p.skipLint -and -not $SkipLint) { $script:SkipLint = [switch]$true }

  Write-Log "Using preset: $lower ($($p.description))" 'INFO'
  Write-Log "  Platforms: $($p.platforms -join ', ')" 'INFO'
  Write-Log "  Architectures: $($p.architectures -join ', ')" 'INFO'
}

if ($Help) { Show-Help; exit 0 }
if ($ListPresets) { Show-Presets; exit 0 }

if ($Preset) {
  Resolve-Preset $Preset
}

if (-not $Platforms) {
  $Platforms = if ($Preset) { @() } else { @('windows') }
}
$Platforms = Split-CommaValues $Platforms

if (-not $Architectures) {
  $Architectures = if ($Preset) { @() } else { @('x64') }
}
$Architectures = Split-CommaValues $Architectures

if ($Platforms.Count -eq 0 -and -not $Preset) {
  $Platforms = @('windows')
}
if ($Architectures.Count -eq 0 -and -not $Preset) {
  $Architectures = @('x64')
}

function Test-Dependency {
  param([string]$Name, [string]$Command)
  if (-not (Get-Command $Command -ErrorAction SilentlyContinue)) {
    Write-Log "$Name not found. Please install $Name." 'ERROR'
    exit 1
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
    & go build -trimpath -ldflags $script:MspLdflags -o $OutPath ./cmd/msp
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

Test-Dependency 'Go' 'go'

Invoke-Step 'Build Frontend' {
  if (-not (Get-Command bun -ErrorAction SilentlyContinue)) {
    Write-Log 'bun not found. Please install bun: https://bun.sh/docs/installation' 'ERROR'
    throw "bun is not installed. Please install bun: https://bun.sh/docs/installation"
  }

  Push-Location (Join-Path $root 'web')
  try {
    if (-not (Test-Path 'node_modules')) {
      Write-Log 'Installing bun dependencies...' 'INFO'
      bun install
      if ($LASTEXITCODE -ne 0) { throw "bun install failed. exitCode=$LASTEXITCODE" }
    }
    Write-Log 'Building frontend...' 'INFO'
    bun run build
    if ($LASTEXITCODE -ne 0) { throw "bun run build failed. exitCode=$LASTEXITCODE" }
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

Invoke-Step 'Cross Build Artifacts' {
  $binRoot = Join-Path $root 'bin'
  $chkRoot = Join-Path $root 'checksums'

  $buildConfigs = @(
    @{ Platform = 'linux';   Arch = 'amd64';   OutName = 'msp-linux-amd64' },
    @{ Platform = 'linux';   Arch = 'arm64';   OutName = 'msp-linux-arm64' },
    @{ Platform = 'linux';   Arch = 'arm';     GOARM = '7'; OutName = 'msp-linux-armv7' },
    @{ Platform = 'linux';   Arch = 'loong64'; OutName = 'msp-linux-loong64' },
    @{ Platform = 'darwin';  Arch = 'amd64';   OutName = 'msp-darwin-amd64' },
    @{ Platform = 'darwin';  Arch = 'arm64';   OutName = 'msp-darwin-arm64' },
    @{ Platform = 'windows'; Arch = 'amd64';   OutName = 'msp-windows-amd64.exe' },
    @{ Platform = 'windows'; Arch = '386';     OutName = 'msp-windows-386.exe' }
  )

  # 收集需要构建的目标
  $buildItems = @()
  foreach ($cfg in $buildConfigs) {
    $platform = $cfg.Platform
    $arch = $cfg.Arch
    $outName = $cfg.OutName
    $goarm = $cfg.GOARM

    $shouldBuild = $false
    if ($platform -eq 'linux' -and $arch -eq 'amd64') {
      $shouldBuild = (ShouldBuild 'linux' 'amd64') -or (ShouldBuild 'linux' 'x64')
    }
    elseif ($platform -eq 'linux' -and $arch -eq 'arm64') {
      $shouldBuild = (ShouldBuild 'linux' 'arm64')
    }
    elseif ($platform -eq 'linux' -and $arch -eq 'arm') {
      $shouldBuild = (ShouldBuild 'arm' 'v7')
    }
    elseif ($platform -eq 'linux' -and $arch -eq 'loong64') {
      $shouldBuild = (ShouldBuild 'linux' 'loong64')
    }
    elseif ($platform -eq 'darwin' -and $arch -eq 'amd64') {
      $shouldBuild = (ShouldBuild 'macos' 'amd64') -or (ShouldBuild 'macos' 'x64')
    }
    elseif ($platform -eq 'darwin' -and $arch -eq 'arm64') {
      $shouldBuild = (ShouldBuild 'macos' 'arm64')
    }
    elseif ($platform -eq 'windows' -and $arch -eq 'amd64') {
      $shouldBuild = (ShouldBuild 'windows' 'amd64') -or (ShouldBuild 'windows' 'x64')
    }
    elseif ($platform -eq 'windows' -and $arch -eq '386') {
      $shouldBuild = (ShouldBuild 'windows' '386') -or (ShouldBuild 'windows' 'x86')
    }

    if ($shouldBuild) {
      $buildItems += [PSCustomObject]@{
        Platform = $platform
        Arch     = $arch
        OutPath  = Join-Path $binRoot "$platform/$arch/$outName"
        ChkPath  = Join-Path $chkRoot "$outName.sha256"
        GOARM    = $goarm
      }
    }
  }

  if ($buildItems.Count -eq 0) {
    Write-Log 'No targets to build.' 'WARN'
    return
  }

  $maxParallel = 4
  Write-Log "Building $($buildItems.Count) target(s) with up to $maxParallel parallel jobs" 'INFO'

  # 使用 Start-Job 分批并发，兼容 PowerShell 5.1 和 7+
  for ($i = 0; $i -lt $buildItems.Count; $i += $maxParallel) {
    $end = [Math]::Min($i + $maxParallel - 1, $buildItems.Count - 1)
    $batch = $buildItems[$i..$end]
    $jobs = @()

    foreach ($item in $batch) {
      $sb = {
        param($Root, $Platform, $Arch, $OutPath, $ChkPath, $GOARM, $Ldflags)
        $env:GOOS = $Platform
        $env:GOARCH = $Arch
        $env:CGO_ENABLED = '0'
        if ($GOARM) { $env:GOARM = $GOARM }
        else { Remove-Item Env:GOARM -ErrorAction SilentlyContinue }

        Push-Location $Root
        try {
          $dir = Split-Path $OutPath -Parent
          if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }

          & go build -trimpath -ldflags $Ldflags -o $OutPath ./cmd/msp
          if ($LASTEXITCODE -ne 0) { throw "go build failed for $Platform/$Arch" }

          $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $OutPath
          $line = $hash.Hash + '  ' + (Split-Path $OutPath -Leaf)
          $chkDir = Split-Path $ChkPath -Parent
          if (-not (Test-Path $chkDir)) { New-Item -ItemType Directory -Force -Path $chkDir | Out-Null }
          $line | Out-File -FilePath $ChkPath -Encoding ascii

          "OK:$Platform/$Arch"
        }
        finally {
          Pop-Location
        }
      }
      $job = Start-Job -ScriptBlock $sb -ArgumentList $root, $item.Platform, $item.Arch, $item.OutPath, $item.ChkPath, $item.GOARM, $script:MspLdflags
      $jobs += $job
    }

    $jobs | Wait-Job
    foreach ($job in $jobs) {
      if ($job.State -eq 'Failed') {
        $err = $job.ChildJobs[0].JobStateInfo.Reason.Message
        $jobs | Remove-Job -Force -ErrorAction SilentlyContinue
        throw "Build failed: $err"
      }
      $result = Receive-Job -Job $job
      if ($result -match '^OK:') {
        Write-Log "Built: $($result -replace '^OK:','')" 'SUCCESS'
      }
      Remove-Job -Job $job
    }
  }
}

Write-Log 'Build completed.' 'SUCCESS'
