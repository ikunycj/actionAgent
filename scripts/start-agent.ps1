param(
  [string]$BinaryPath = "..\agent\actionagentd.exe",
  [string]$ConfigPath = "..\agent\actionAgent.json",
  [string]$Addr = ""
)

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$bin = [System.IO.Path]::GetFullPath((Join-Path $scriptDir $BinaryPath))
$config = [System.IO.Path]::GetFullPath((Join-Path $scriptDir $ConfigPath))

if (!(Test-Path $bin)) {
  Write-Error "Binary not found: $bin"
  exit 1
}

$configDir = Split-Path -Parent $config
if (!(Test-Path $configDir)) {
  New-Item -ItemType Directory -Force -Path $configDir | Out-Null
}

$args = @("--config", $config)
if ($Addr -ne "") {
  $args += @("--addr", $Addr)
}

& $bin @args