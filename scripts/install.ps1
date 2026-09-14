<#
.SYNOPSIS
  Installs ctx on Windows (PowerShell 5.1+).
.EXAMPLE
  powershell -ExecutionPolicy Bypass -File install.ps1
  powershell -ExecutionPolicy Bypass -File install.ps1 -Version v0.1.0
#>
param(
  [string]$Version = "latest",
  [string]$Repo = "AnshGajera/CTX"
)

$ErrorActionPreference = "Stop"

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  "ARM64" { "arm64" }
  default { throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

if ($Version -eq "latest") {
  $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $release.tag_name
}
Write-Host "Installing ctx $Version (windows/$arch)..."

$asset = "ctx_${($Version -replace '^v','')}_windows_${arch}.zip"
if ($arch -eq "arm64") {
  # No windows/arm64 build published; fall back to amd64 under emulation.
  Write-Warning "No windows/arm64 asset published; using amd64 (runs via emulation)."
  $arch = "amd64"
  $asset = "ctx_${($Version -replace '^v','')}_windows_amd64.zip"
}
$base = "https://github.com/$Repo/releases/download/$Version"

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("ctx-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Invoke-WebRequest -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
  Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt")

  $expected = (Select-String -Path (Join-Path $tmp "checksums.txt") -Pattern $asset | ForEach-Object { $_.Line.Split()[0] })
  $actual = (Get-FileHash -Path (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
  if ($expected -and ($expected.ToLower() -ne $actual)) {
    throw "Checksum mismatch for $asset. Expected $expected, got $actual."
  }

  $binDir = Join-Path $HOME ".ctx\bin"
  New-Item -ItemType Directory -Force -Path $binDir | Out-Null
  Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath $binDir -Force

  $path = [Environment]::GetEnvironmentVariable("Path", "User")
  if ($path -notlike "*$binDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$path;$binDir", "User")
    $env:Path += ";$binDir"
    Write-Host "Added $binDir to user PATH (restart shell to pick up)."
  }

  $cfgDir = Join-Path $HOME ".config\ctx"
  New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
  $cfg = Join-Path $cfgDir "config.toml"
  if (-not (Test-Path $cfg)) {
    @'
[core]
api_url = "https://api.ctx.dev"
auto_sync = true
watch_mode = true
ml_url = "http://localhost:8001"

[extraction]
architecture = true
api_endpoints = true
database_schema = true
dependencies = true
business_rules = true
env_vars = true

[privacy]
exclude_patterns = ["*.pem", "*.key", "secrets/*"]
redact_values = true
hash_identifiers = false

[sync]
interval_seconds = 300
on_git_commit = true
on_file_save = false
'@ | Set-Content -Path $cfg -Encoding UTF8
  }
  Write-Host "Installed to $binDir\ctx.exe"
  Write-Host "Next: ctx init; ctx extract; ctx status"
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
