# Veil installer — Windows (PowerShell)
#
# Usage:
#   irm https://veil.sh/install.ps1 | iex
#
# Environment variables:
#   $env:VEIL_VERSION     - specific version tag, e.g. v0.1.0 (default: latest)
#   $env:VEIL_INSTALL_DIR - installation directory (default: %USERPROFILE%\.veil\bin)
#   $env:VEIL_DOWNLOAD_BASE - release asset base URL override for CI smoke tests

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo        = "PAIArtCom/Veil"
$BinName     = "veil"
$ApiUrl      = "https://api.github.com/repos/$Repo/releases/latest"
$ReleasesUrl = "https://github.com/$Repo/releases"

# architecture
$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "ARM64"  { "arm64" }
  "AMD64"  { "amd64" }
  "x86"    { throw "Veil requires a 64-bit system." }
  default  { if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { throw "Veil requires a 64-bit system." } }
}

# version
$Version = $env:VEIL_VERSION
if (-not $Version) {
  try {
    $Release = Invoke-RestMethod -Uri $ApiUrl -Headers @{ "User-Agent" = "veil-installer/1.0" }
    $Version = $Release.tag_name
  } catch {
    throw "Failed to resolve latest Veil version: $_"
  }
}
if (-not $Version) { throw "Could not determine Veil version." }

# install dir
$InstallDir = $env:VEIL_INSTALL_DIR
if (-not $InstallDir) { $InstallDir = Join-Path $env:USERPROFILE ".veil\bin" }
[void](New-Item -ItemType Directory -Force -Path $InstallDir)

Write-Host "Installing Veil $Version (windows/$Arch) -> $InstallDir"

# download
$Artifact    = "${BinName}-${Version}-windows-${Arch}.exe"
$DownloadBase = $env:VEIL_DOWNLOAD_BASE
if (-not $DownloadBase) { $DownloadBase = "$ReleasesUrl/download/$Version" }
$DownloadUrl = "$DownloadBase/$Artifact"
$ChecksumUrl = "$DownloadBase/checksums.txt"

$Tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
[void](New-Item -ItemType Directory -Force -Path $Tmp)

try {
  Write-Host "Downloading $Artifact..."
  Invoke-WebRequest -Uri $DownloadUrl -OutFile "$Tmp\${BinName}.exe" -UseBasicParsing

  Write-Host "Verifying checksum..."
  Invoke-WebRequest -Uri $ChecksumUrl -OutFile "$Tmp\checksums.txt" -UseBasicParsing

  # checksum
  $Line = Get-Content "$Tmp\checksums.txt" |
          Where-Object { (($_ -split '\s+')[1]) -eq $Artifact } |
          Select-Object -First 1
  if (-not $Line) { throw "No checksum entry for $Artifact in checksums.txt" }

  $Expected = ($Line -split '\s+')[0].ToLower()
  $Actual   = (Get-FileHash "$Tmp\${BinName}.exe" -Algorithm SHA256).Hash.ToLower()
  if ($Actual -ne $Expected) {
    throw "Checksum mismatch for ${Artifact}`n  expected: $Expected`n  got:      $Actual"
  }

  # install
  $Dest = Join-Path $InstallDir "${BinName}.exe"
  Copy-Item "$Tmp\${BinName}.exe" $Dest -Force

} finally {
  Remove-Item $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# PATH
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
$Dirs     = $UserPath -split ";"
if ($Dirs -notcontains $InstallDir) {
  $NewPath = ($Dirs + $InstallDir | Where-Object { $_ }) -join ";"
  [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
  $env:PATH = "$env:PATH;$InstallDir"
  Write-Host ""
  Write-Host "Added $InstallDir to your PATH. Restart your terminal to apply."
}

Write-Host ""
Write-Host "Veil $Version installed to $Dest"
Write-Host ""
& $Dest version
