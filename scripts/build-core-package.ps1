param(
  [string]$OutputDir = "out/core-package",
  [switch]$SkipWebBuild
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$webDir = Join-Path $repoRoot "web"
$agentDir = Join-Path $repoRoot "agent"
$packageDir = Join-Path $repoRoot $OutputDir
$binaryPath = Join-Path $packageDir "actionagentd.exe"
$webBundleDir = Join-Path $packageDir "webui"

if (-not $SkipWebBuild) {
  Push-Location $webDir
  try {
    npm run build
  } finally {
    Pop-Location
  }
}

if (Test-Path $packageDir) {
  Remove-Item -Path $packageDir -Recurse -Force
}

New-Item -ItemType Directory -Path $webBundleDir -Force | Out-Null

Push-Location $agentDir
try {
  go build -buildvcs=false -o $binaryPath ./cmd/actionagentd
} finally {
  Pop-Location
}

Copy-Item (Join-Path $webDir "dist\\*") -Destination $webBundleDir -Recurse -Force
Copy-Item (Join-Path $agentDir "actionAgent.json") -Destination $packageDir -Force

Write-Host "Core package ready:" $packageDir
Write-Host "Binary:" $binaryPath
Write-Host "WebUI:" $webBundleDir
