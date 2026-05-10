param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$TaskParts
)

$ErrorActionPreference = "Stop"

$task = ($TaskParts -join " ").Trim()
if (-not $task) {
    $task = $env:DIVINITY_TASK
}
if (-not $task) {
    $task = "No task provided."
}

$workspace = $env:DIVINITY_WORKTREE
if (-not $workspace) {
    $workspace = (Get-Location).Path
}

$trackedFiles = git ls-files 2>$null | Select-Object -First 30
$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

$report = @(
    "# Divinity Agentic Smoke Test"
    ""
    "Generated: $timestamp"
    "Agent: $env:DIVINITY_AGENT"
    "Run: $env:DIVINITY_RUN_ID"
    "Workspace: $workspace"
    ""
    "## Task"
    ""
    $task
    ""
    "## What this proves"
    ""
    "- The agent ran inside an isolated Git worktree."
    "- The agent could inspect repository files."
    "- The agent could create or modify files."
    "- Divinity can capture the resulting diff."
    ""
    "## First tracked files"
    ""
)

foreach ($file in $trackedFiles) {
    $report += "- $file"
}

if (-not $trackedFiles) {
    $report += "- No tracked files found."
}

$report | Set-Content -LiteralPath "DIVINITY_AGENTIC_SMOKE.md" -Encoding UTF8

Write-Output "Created DIVINITY_AGENTIC_SMOKE.md"
