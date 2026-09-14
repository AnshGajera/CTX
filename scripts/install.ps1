<#
.SYNOPSIS
  Installs ctx on Windows (PowerShell 5.1+). Resolves the exact asset
  name via the GitHub Releases API.
.EXAMPLE
  powershell -ExecutionPolicy Bypass -File install.ps1
  powershell -ExecutionPolicy Bypass -File install.ps1 -Version v0.1.0
#>
param(
  [string]$Version = "latest",
  [string]$Repo = "AnshGajera/CTX"
)

$ErrorActionPreference = "Stop"

$cpu = $env:PROCESSOR_ARCHITECTURE
$archPatterns = if ($cpu -eq "ARM64") { @("arm64", "aarch64") } else { @("amd64", "x86_64") }

if ($Version -eq "latest") {
  $Version = (Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest").tag_name
}
Write-Host "Installing ctx $Version (windows/$cpu)..."

$rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/tags/$Version"
$asset = $rel.assets | Where-Object {
  $_.name -match 'windows' -and $_.name -match ($archPatterns -join '|') -and $_.name -match '\.(zip|tar\.gz)$'
} | Select-Object -First 1
if (-not $asset) {
  Write-Error "No windows/$cpu asset in $Version. Available:`n$($rel.assets.name -join "`n")"
  exit 1
}
$checksums = $rel.assets | Where-Object { $_.name -match 'checksum' } | Select-Object -First 1
Write-Host "Asset: $($asset.name)"

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("ctx-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  $archive = Join-Path $tmp $asset.name
  Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $archive

  if ($checksums) {
    $sumFile = Join-Path $tmp $checksums.name
    Invoke-WebRequest -Uri $checksums.browser_download_url -OutFile $sumFile
    $line = Select-String -Path $sumFile -Pattern ([regex]::Escape($asset.name)) | Select-Object -First 1
    if ($line) {
      $expected = ($line.Line -split '\s+')[0].ToLower()
      $actual = (Get-FileHash -Path $archive -Algorithm SHA256).Hash.ToLower()
      if ($expected -ne $actual) {
        throw "Checksum mismatch for $($asset.name). Expected $expected, got $actual."
      }
      Write-Host "Checksum OK."
    }
  }

  $binDir = Join-Path $HOME ".ctx\bin"
  New-Item -ItemType Directory -Force -Path $binDir | Out-Null
  if ($archive -match '\.zip$') {
    Expand-Archive -Path $archive -DestinationPath $binDir -Force
  } else {
    tar -xzf $archive -C $binDir
  }

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
