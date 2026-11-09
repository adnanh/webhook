#Requires -Version 5.1
# Run the installer's actual hash/ZIP functions without downloading or installing.
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$InstallerPath = Join-Path $PSScriptRoot "install.ps1"
$Tokens = $null
$ParseErrors = $null
$Ast = [Management.Automation.Language.Parser]::ParseFile($InstallerPath, [ref] $Tokens, [ref] $ParseErrors)
if ($ParseErrors.Count -ne 0) { throw "Cannot parse install.ps1: $ParseErrors" }
$Names = @("Get-ArchiveSha256", "Expand-ReleaseZip")
$Functions = @($Ast.FindAll({
    param($Node)
    $Node -is [Management.Automation.Language.FunctionDefinitionAst] -and $Node.Name -in $Names
}, $false))
if ($Functions.Count -ne $Names.Count) { throw "Installer hash/ZIP functions are missing" }
foreach ($Definition in $Functions) { . ([scriptblock]::Create($Definition.Extent.Text)) }

# These guards turn unintended module dependencies into deterministic failures.
function Get-FileHash { throw "Unexpected Get-FileHash dependency" }
function Expand-Archive { throw "Unexpected Expand-Archive dependency" }

$TempDir = Join-Path $ProjectRoot ("build/powershell-tests-" + [Guid]::NewGuid())
[IO.Directory]::CreateDirectory($TempDir) | Out-Null
try {
    $InputPath = Join-Path $TempDir "input [literal] with spaces.txt"
    [IO.File]::WriteAllBytes($InputPath, [Text.Encoding]::UTF8.GetBytes("abc"))
    $ExpectedHash = "BA7816BF8F01CFEA414140DE5DAE2223B00361A396177A9CB410FF61F20015AD"
    if ((Get-ArchiveSha256 $InputPath) -cne $ExpectedHash) { throw "SHA256 does not match the known test vector" }
    # The hash function must release the file before installation/cleanup.
    $Exclusive = [IO.File]::Open($InputPath, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
    $Exclusive.Dispose()

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $SourceDir = Join-Path $TempDir "source with spaces"
    $BundleDir = Join-Path $SourceDir "webhook-windows-amd64"
    [IO.Directory]::CreateDirectory($BundleDir) | Out-Null
    [IO.File]::WriteAllText((Join-Path $BundleDir "webhook.exe"), "fixture binary")
    $ZipPath = Join-Path $TempDir "release [literal].zip"
    [IO.Compression.ZipFile]::CreateFromDirectory($SourceDir, $ZipPath)
    $Destination = Join-Path $TempDir "extracted with spaces"
    Expand-ReleaseZip $ZipPath $Destination
    $ExtractedPath = Join-Path $Destination "webhook-windows-amd64/webhook.exe"
    if ([IO.File]::ReadAllText($ExtractedPath) -cne "fixture binary") { throw "Extracted ZIP content does not match" }

    $Rejected = $false
    try { Expand-ReleaseZip $InputPath (Join-Path $TempDir "corrupt") } catch {
        if ($_.Exception.Message -notlike "Failed to extract ZIP archive*") { throw }
        $Rejected = $true
    }
    if (-not $Rejected) { throw "Corrupt archive was not rejected" }
    Write-Host "PowerShell $($PSVersionTable.PSVersion): SHA256, file cleanup and ZIP tests passed."
} finally {
    [IO.Directory]::Delete($TempDir, $true)
}
