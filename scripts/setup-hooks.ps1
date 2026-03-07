param(
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$gitArgs = @('-C', $RepoRoot, 'config', 'core.hooksPath', '.githooks')
& git @gitArgs

if ($LASTEXITCODE -ne 0) {
  Write-Error 'Failed to configure core.hooksPath.'
  exit $LASTEXITCODE
}

Write-Host 'Configured core.hooksPath=.githooks for repository:' $RepoRoot
