#Requires -Version 5.1
[CmdletBinding()]
param(
    [string] $Repo = $env:WEBHOOK_REPO,
    [string] $InstallDir = $env:WEBHOOK_INSTALL_DIR,
    [string] $Version = $env:WEBHOOK_VERSION,
    [string] $BinaryName = $env:WEBHOOK_BINARY_NAME
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($Repo)) { $Repo = "xtulnx/webhook" }
if ([string]::IsNullOrWhiteSpace($InstallDir)) { $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\webhook" }
if ([string]::IsNullOrWhiteSpace($Version)) { $Version = "latest" }
if ([string]::IsNullOrWhiteSpace($BinaryName)) { $BinaryName = "webhook.exe" }
if ($Repo -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') { throw "Repo must be owner/repo" }
if ($BinaryName -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') { throw "BinaryName must be a filename starting with a letter or number" }
if (-not $BinaryName.EndsWith(".exe", [StringComparison]::OrdinalIgnoreCase)) { $BinaryName += ".exe" }
$InstallDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($InstallDir)

# PROCESSOR_ARCHITEW6432 identifies the native OS when using 32-bit PowerShell.
$Architecture = $env:PROCESSOR_ARCHITEW6432
if ([string]::IsNullOrWhiteSpace($Architecture)) { $Architecture = $env:PROCESSOR_ARCHITECTURE }
switch ($Architecture) {
    "AMD64" { $Arch = "amd64" }
    "ARM64" { $Arch = "arm64" }
    "x86" { $Arch = "386" }
    default { throw "Unsupported architecture: $Architecture" }
}

function Get-Download([string] $Uri, [string] $OutFile) {
    for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $OutFile -TimeoutSec 300
            return
        } catch {
            if ($Attempt -eq 3) { throw "Failed to download $Uri. $($_.Exception.Message)" }
            Start-Sleep -Seconds $Attempt
        }
    }
}

function Get-ArchiveSha256([string] $Archive) {
    # Use .NET directly: script-based cmdlets may be unavailable when Windows
    # PowerShell inherits a PSModulePath configured for PowerShell 7.
    $Hasher = [Security.Cryptography.SHA256]::Create()
    $Stream = $null
    try {
        $Stream = [IO.File]::OpenRead($Archive)
        return [BitConverter]::ToString($Hasher.ComputeHash($Stream)).Replace("-", "")
    } finally {
        if ($Stream) { $Stream.Dispose() }
        $Hasher.Dispose()
    }
}

function Expand-ReleaseZip([string] $Archive, [string] $Destination) {
    try {
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [IO.Compression.ZipFile]::ExtractToDirectory($Archive, $Destination)
    } catch {
        throw "Failed to extract ZIP archive $Archive. $($_.Exception.Message)"
    }
}

$TempDir = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
$Staged = $null
$PreviousTls = [Net.ServicePointManager]::SecurityProtocol
[Net.ServicePointManager]::SecurityProtocol = $PreviousTls -bor [Net.SecurityProtocolType]::Tls12
New-Item -ItemType Directory -Path $TempDir | Out-Null
try {
    # Pin latest once; a release published during installation cannot mix assets.
    if ($Version -eq "latest") {
        $MetadataPath = Join-Path $TempDir "release.json"
        Get-Download "https://api.github.com/repos/$Repo/releases/latest" $MetadataPath
        $Version = (Get-Content -LiteralPath $MetadataPath -Raw | ConvertFrom-Json).tag_name
    } elseif (-not $Version.StartsWith("v")) {
        $Version = "v$Version"
    }
    if ($Version -cnotmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$') { throw "Invalid release version (expected vX.Y.Z)" }

    # Prefer native ZIP extraction; older releases only provided tar.gz.
    $Asset = "webhook-windows-$Arch.zip"
    $BaseUrl = "https://github.com/$Repo/releases/download/$Version"
    Write-Host "Downloading $Repo $Version for windows/$Arch..."
    $ChecksumsPath = Join-Path $TempDir "checksums.txt"
    Get-Download "$BaseUrl/checksums.txt" $ChecksumsPath
    $Checksums = @(Get-Content -LiteralPath $ChecksumsPath)
    $ZipLines = @($Checksums | Where-Object { $_ -cmatch "\s$([regex]::Escape($Asset))$" })
    if ($ZipLines.Count -eq 0) { $Asset = "webhook-windows-$Arch.tar.gz" }
    $ChecksumLines = @($Checksums | Where-Object { $_ -cmatch "^[0-9a-fA-F]{64}  $([regex]::Escape($Asset))$" })
    if ($ChecksumLines.Count -ne 1) { throw "Expected exactly one checksum for $Asset" }
    $Archive = Join-Path $TempDir $Asset
    Get-Download "$BaseUrl/$Asset" $Archive
    $ExpectedHash = ($ChecksumLines[0] -split "\s+")[0]
    $ActualHash = Get-ArchiveSha256 $Archive
    if ($ActualHash -ne $ExpectedHash) { throw "Checksum mismatch for $Asset" }

    $ExtractDir = Join-Path $TempDir "extracted"
    if ($Asset.EndsWith(".zip")) {
        Expand-ReleaseZip $Archive $ExtractDir
    } else {
        $Tar = Get-Command tar.exe -ErrorAction SilentlyContinue
        if (-not $Tar) { throw "This older release only provides tar.gz; install tar.exe or select a newer release with ZIP assets" }
        New-Item -ItemType Directory -Path $ExtractDir | Out-Null
        & $Tar.Source -xzf $Archive -C $ExtractDir "webhook-windows-$Arch/webhook.exe"
        if ($LASTEXITCODE -ne 0) { throw "Failed to extract $Asset" }
    }
    $Source = Join-Path $TempDir "extracted\webhook-windows-$Arch\webhook.exe"
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) { throw "Archive does not contain webhook.exe" }
    $ReportedVersion = & $Source -version
    if ($LASTEXITCODE -ne 0 -or $ReportedVersion -cne "webhook version $($Version.Substring(1))") { throw "Downloaded binary reported an unexpected version: $ReportedVersion" }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $Target = Join-Path $InstallDir $BinaryName
    $Staged = Join-Path $InstallDir (".$BinaryName.$([Guid]::NewGuid()).tmp")
    Copy-Item -LiteralPath $Source -Destination $Staged
    try {
        if (Test-Path -LiteralPath $Target) {
            # PowerShell converts $null to an empty string for .NET string parameters.
            [IO.File]::Replace($Staged, $Target, [NullString]::Value)
        } else {
            [IO.File]::Move($Staged, $Target)
        }
    } catch {
        throw "Cannot replace $Target. Stop the running webhook service/process and retry. $($_.Exception.Message)"
    }
    $Staged = $null
    Write-Host "Installed $ReportedVersion to $Target"

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Paths = @($UserPath -split ";" | Where-Object { $_ })
    if ($Paths -notcontains $InstallDir) {
        [Environment]::SetEnvironmentVariable("Path", (($Paths + $InstallDir) -join ";"), "User")
        Write-Host "Added $InstallDir to user PATH. Open a new terminal to use webhook directly."
    }
    if (($env:Path -split ";") -notcontains $InstallDir) { $env:Path += ";$InstallDir" }
    Write-Host "Restart any running webhook service to use the installed version."
} finally {
    [Net.ServicePointManager]::SecurityProtocol = $PreviousTls
    if ($Staged -and (Test-Path -LiteralPath $Staged)) { Remove-Item -LiteralPath $Staged -Force -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
