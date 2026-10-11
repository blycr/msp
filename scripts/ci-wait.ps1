#!/usr/bin/env pwsh
# ci-wait.ps1 - poll GitHub Actions until the newest run for a ref reaches a
# terminal state, then exit immediately.
#
# Why this exists: fixed `Start-Sleep` waits are wrong in both directions. They
# overshoot (burning wall-clock time on a run that finished long ago) and they
# undershoot (spending another whole agent turn re-checking). Polling with a
# tight interval plus a hard deadline returns as soon as the answer exists.
#
# Usage:
#   pwsh -File scripts/ci-wait.ps1 -Ref main
#   pwsh -File scripts/ci-wait.ps1 -Ref main -Event push -Workflow check
#   pwsh -File scripts/ci-wait.ps1 -Ref v1.12.3 -Event push -Workflow release
#   pwsh -File scripts/ci-wait.ps1 -Ref main -TimeoutSec 1800
#
# Exit codes: 0 = terminal state reached (print it; NOT necessarily success),
#             2 = deadline hit while still queued/in progress,
#             3 = no run ever appeared; the ref does not match the workflow's
#                 trigger (e.g. a tag that does not match `tags: v*`), so waiting
#                 longer cannot help.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Ref,
    [string]$Workflow = '',
    [string]$Event = '',
    [int]$TimeoutSec = 1200,
    [int]$IntervalSec = 25,
    # How long a ref may produce no run at all before we call it a
    # never-triggering ref. GitHub queue lag is tens of seconds, not minutes.
    [int]$NoRunGraceSec = 240
)

$ErrorActionPreference = 'Stop'
$repo = 'blycr/msp'
$repoPath = Split-Path -Parent $PSScriptRoot
$envLine = ''

function Get-GhEnv {
    if ($script:envLine) { return $script:envLine }
    Push-Location $repoPath
    try {
        $lines = & gh api "repos/$repo/actions/runs?per_page=1" 2>&1
        $token = ''
        foreach ($l in $lines) {
            if ($l -match 'HTTP_TOKEN:\s*(\S+)') { $token = $Matches[1] }
        }
        if (-not $token) { throw "could not read the GitHub token from 'gh api -i' output (gh not authenticated?)" }
        $script:envLine = "HTTP_TOKEN:$token"
    } finally { Pop-Location }
    return $script:envLine
}

$deadline = (Get-Date).AddSeconds($TimeoutSec)
$started = Get-Date
$first = $true

while ($true) {
    if (-not $first) { Start-Sleep -Seconds $IntervalSec }
    $first = $false

    $uri = "repos/$repo/actions/runs?per_page=20&branch=$Ref"
    if ($Event) { $uri += "&event=$Event" }

    $resp = & gh api $uri | ConvertFrom-Json
    $run = $resp.workflow_runs | Where-Object { $_.head_branch -eq $Ref } | Select-Object -First 1
    if ($Workflow) {
        $run = $resp.workflow_runs | Where-Object { $_.head_branch -eq $Ref -and $_.name -eq $Workflow } | Select-Object -First 1
    }

    $elapsed = [int]((Get-Date) - $started).TotalSeconds

    if (-not $run) {
        Write-Output "[$elapsed s] no run yet for ref '$Ref' (queue lag); still polling"
        if ($elapsed -ge $NoRunGraceSec) {
            Write-Output "GIVING UP: no run for '$Ref' after $elapsed s."
            Write-Output "A ref that matches no workflow trigger never produces a run. Check the"
            Write-Output "workflow's `on:` filters (e.g. release.yml only fires for tags matching 'v*')."
            exit 3
        }
    } elseif ($run.status -ne 'completed') {
        Write-Output "[$elapsed s] $($run.name) [$($run.status)] still running"
    } else {
        Write-Output "RESULT: $($run.name) [$($run.event)] for '$Ref' -> $($run.conclusion)"
        Write-Output "run id: $($run.id)"
        Write-Output "url: $($run.html_url)"

        $jobs = & gh api "repos/$repo/actions/runs/$($run.id)/jobs" | ConvertFrom-Json
        foreach ($j in $jobs.jobs) {
            $line = '  {0,-26} {1}' -f $j.name, $j.conclusion
            Write-Output $line
        }
        $notGreen = $jobs.jobs | Where-Object { $_.conclusion -ne 'success' }
        if ($notGreen) {
            Write-Output ''
            Write-Output 'NON-SUCCESS JOBS:'
            foreach ($j in $notGreen) {
                Write-Output "  $($j.name) -> $($j.conclusion)  $($j.html_url)"
            }
        }
        exit 0
    }

    if ((Get-Date) -ge $deadline) {
        Write-Output "TIMEOUT after $TimeoutSec s; latest run still '$($run.status)'"
        if ($run) { Write-Output "url: $($run.html_url)" }
        exit 2
    }
}
