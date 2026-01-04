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

$exePath = Join-Path $root 'msp.exe'

Invoke-Step ("Check previous exe: " + $exePath) {
  if (-not (Test-Path -LiteralPath $exePath)) {
    Write-Log 'Previous exe not found. Skip stop/delete.' 'INFO'
    return
  }
  $exeFull = (Resolve-Path -LiteralPath $exePath).Path
  Write-Log ("Previous exe found: " + $exeFull) 'INFO'

  $procs = @()
  try { $procs = Get-Process -Name 'msp' -ErrorAction SilentlyContinue } catch { $procs = @() }

  foreach ($p in $procs) {
    $pPath = $null
    try { $pPath = $p.Path } catch { $pPath = $null }
    if (-not $pPath) { continue }

    $pFull = $null
    try { $pFull = (Resolve-Path -LiteralPath $pPath).Path } catch { $pFull = $pPath }

    if ($pFull -ne $exeFull) { continue }

    Write-Log ("Previous process is running: PID=" + $p.Id + " Path=" + $pFull) 'WARN'
    try {
      Stop-Process -Id $p.Id -Force -ErrorAction Stop
      try { $p.WaitForExit(3000) } catch {}
      Write-Log ("Process stopped: PID=" + $p.Id) 'INFO'
    } catch {
      Write-Log ("Stop process failed: PID=" + $p.Id + " " + $_.Exception.Message) 'ERROR'
      throw
    }
  }

  $retries = 0
  $maxRetries = 10
  while ($true) {
    try {
      Remove-Item -LiteralPath $exeFull -Force -ErrorAction Stop
      Write-Log ("Previous exe deleted: " + $exeFull) 'INFO'
      break
    } catch {
      $retries++
      if ($retries -ge $maxRetries) {
        Write-Log ("Delete previous exe failed after $maxRetries retries: " + $_.Exception.Message) 'ERROR'
        throw
      }
      Write-Log ("Delete failed, retrying... ($retries/$maxRetries)") 'WARN'
      Start-Sleep -Milliseconds 500
    }
  }
}

Invoke-Step 'go test ./...' {
  Push-Location $root
  try {
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw ("go test failed. exitCode=" + $LASTEXITCODE) }
  } finally {
    Pop-Location
  }
}

Invoke-Step 'go build -o msp.exe ./cmd/msp' {
  Push-Location $root
  try {
    & go build -o msp.exe ./cmd/msp
    if ($LASTEXITCODE -ne 0) { throw ("go build failed. exitCode=" + $LASTEXITCODE) }
  } finally {
    Pop-Location
  }
}

Write-Log 'Build completed.' 'INFO'

