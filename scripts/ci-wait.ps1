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
#   pwsh -File scripts/ci-wait.ps1 -Ref v1.12.3 -Event push -Workflow release
#   pwsh -File scripts/ci-wait.ps1 -Ref main -Event pull_request
#   pwsh -File scripts/ci-wait.ps1 -Ref main -TimeoutSec 1800
#
# Note: check.yml only runs on pull_request and workflow_call (plain branch
# pushes no longer trigger it), so `-Ref main -Event push` will usually find a
# release run or nothing. Use -Event pull_request for PR checks.
#
# Exit codes: 0 = terminal state reached (print it; NOT necessarily success),
#             1 = could not talk to gh / bad response,
#             2 = deadline hit while still queued/in progress,
#             3 = no run ever appeared; the ref does not match the workflow's
#                 trigger (e.g. a tag that does not match `tags: v*`), so waiting
#                 longer cannot help.
#
# Encoding note: GitHub returns UTF-8, but Windows PowerShell's
# [Console]::OutputEncoding defaults to the OEM/ANSI code page (936/GBK on a
# zh-CN box). Piping `gh` output straight into ConvertFrom-Json therefore mangles
# any non-ASCII commit title ("docs: 记录 ...") and the parser dies with
# "unexpected character ... Path 'display_title'". Every gh call below goes
# through Invoke-GhJson, which reads raw bytes and decodes them as UTF-8.

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

# Decode gh's UTF-8 bytes explicitly. Setting [Console]::OutputEncoding also
# helps for any stray native output, but the explicit decode below is what makes
# this correct regardless of the host's code page.
try { [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false) } catch { }

function Invoke-GhJson {
    <#
      Run `gh api <uri>` and return the parsed object, decoding stdout as UTF-8
      by hand so a non-ASCII commit title cannot break ConvertFrom-Json.
      Throws on a non-zero exit code.
    #>
    param([Parameter(Mandatory = $true)][string]$Uri)

    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = 'gh'
    $psi.Arguments = "api $Uri"
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $psi.StandardOutputEncoding = [Text.UTF8Encoding]::new($false)
    $psi.StandardErrorEncoding = [Text.UTF8Encoding]::new($false)

    $p = [Diagnostics.Process]::Start($psi)
    $out = $p.StandardOutput.ReadToEnd()
    $err = $p.StandardError.ReadToEnd()
    $p.WaitForExit()

    if ($p.ExitCode -ne 0) {
        throw "gh api $Uri failed (exit $($p.ExitCode)): $($err.Trim())"
    }
    if (-not $out.Trim()) {
        throw "gh api $Uri returned no output"
    }
    # Strip a UTF-8 BOM if one sneaks in; ConvertFrom-Json chokes on it.
    $out = $out.TrimStart([char]0xFEFF)
    return ($out | ConvertFrom-Json)
}

$deadline = (Get-Date).AddSeconds($TimeoutSec)
$started = Get-Date
$first = $true

while ($true) {
    if (-not $first) { Start-Sleep -Seconds $IntervalSec }
    $first = $false

    $uri = "repos/$repo/actions/runs?per_page=20&branch=$Ref"
    if ($Event) { $uri += "&event=$Event" }

    $resp = Invoke-GhJson -Uri $uri
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
        Write-Output "title: $($run.display_title)"
        Write-Output "url: $($run.html_url)"

        $jobs = Invoke-GhJson -Uri "repos/$repo/actions/runs/$($run.id)/jobs"
        foreach ($j in $jobs.jobs) {
            $line = '  {0,-26} {1}' -f $j.name, $j.conclusion
            Write-Output $line
        }
        $notGreen = $jobs.jobs | Where-Object { $_.conclusion -ne 'success' -and $_.conclusion -ne 'skipped' }
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
