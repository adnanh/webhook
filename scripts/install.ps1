[CmdletBinding()]
param(
    [string] $Repo = $env:WEBHOOK_REPO,
    [string] $InstallDir = $env:WEBHOOK_INSTALL_DIR,
    [string] $Version = $env:WEBHOOK_VERSION,
    [string] $BinaryName = $env:WEBHOOK_BINARY_NAME
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Repo)) {
    $Repo = "xtulnx/webhook"
}
if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\webhook"
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "latest"
}
if ([string]::IsNullOrWhiteSpace($BinaryName)) {
    $BinaryName = "webhook.exe"
}
if (-not $BinaryName.EndsWith(".exe", [StringComparison]::OrdinalIgnoreCase)) {
    $BinaryName = "$BinaryName.exe"
}

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $Arch = "amd64" }
    "ARM64" { $Arch = "arm64" }
    "x86" { $Arch = "386" }
    default {
        throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
    }
}

$Asset = "webhook-windows-$Arch.tar.gz"
if ($Version -eq "latest") {
    $Url = "https://github.com/$Repo/releases/latest/download/$Asset"
    $ChecksumsUrl = "https://github.com/$Repo/releases/latest/download/checksums.txt"
} else {
    $Url = "https://github.com/$Repo/releases/download/$Version/$Asset"
    $ChecksumsUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"
}

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
    $Archive = Join-Path $TempDir $Asset
    Write-Host "Downloading $Repo $Version for windows/$Arch..."
    Invoke-WebRequest -Uri $Url -OutFile $Archive
    $ChecksumsPath = Join-Path $TempDir "checksums.txt"
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $ChecksumsPath

    $ChecksumLine = Get-Content $ChecksumsPath | Where-Object { $_ -match "\s$([regex]::Escape($Asset))$" } | Select-Object -First 1
    if (-not $ChecksumLine) {
        throw "Checksum for $Asset not found"
    }

    $ExpectedHash = ($ChecksumLine -split "\s+")[0].ToLowerInvariant()
    $ActualHash = (Get-FileHash -Algorithm SHA256 -Path $Archive).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ExpectedHash) {
        throw "Checksum mismatch for $Asset"
    }

    tar -xzf $Archive -C $TempDir

    $Source = Join-Path $TempDir "webhook-windows-$Arch\webhook.exe"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path $Source -Destination (Join-Path $InstallDir $BinaryName) -Force

    Write-Host "Installed $BinaryName to $InstallDir"
    & (Join-Path $InstallDir $BinaryName) -version

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Paths = $UserPath -split ";" | Where-Object { $_ }
    if ($Paths -notcontains $InstallDir) {
        [Environment]::SetEnvironmentVariable("Path", (($Paths + $InstallDir) -join ";"), "User")
        Write-Host "Added $InstallDir to the user PATH. Open a new terminal to use webhook directly."
    }
} finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
