# GoThinkDB Installer for Windows (PowerShell)
# Usage: iwr -useb https://raw.githubusercontent.com/leandroasilva/gothinkdb/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

function Write-Header {
    Write-Host "==========================================" -ForegroundColor Blue
    Write-Host "GoThinkDB Installer for Windows" -ForegroundColor Blue
    Write-Host "==========================================" -ForegroundColor Blue
    Write-Host ""
}

function Write-Success {
    param($Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Info {
    param($Message)
    Write-Host "→ $Message" -ForegroundColor Yellow
}

function Write-Error-Custom {
    param($Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

Write-Header

# Detect architecture
$Arch = $env:PROCESSOR_ARCHITECTURE
if ($Arch -eq "AMD64") {
    $ArchName = "amd64"
} elseif ($Arch -eq "ARM64") {
    $ArchName = "arm64"
} else {
    Write-Error-Custom "Unsupported architecture: $Arch"
    exit 1
}

Write-Info "Detected: Windows $ArchName"
Write-Host ""

# Get latest release version
Write-Info "Fetching latest release..."
try {
    $Response = Invoke-RestMethod -Uri "https://api.github.com/repos/leandroasilva/gothinkdb/releases/latest" -Method Get
    $LatestVersion = $Response.tag_name -replace "^v", ""
} catch {
    Write-Error-Custom "Could not fetch latest version"
    exit 1
}

Write-Success "Latest version: v$LatestVersion"
Write-Host ""

# Determine install directory
$InstallDir = "$env:LOCALAPPDATA\GoThinkDB"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Download binary
$BinaryName = "gothinkdb-windows-$ArchName.exe"
$DownloadUrl = "https://github.com/leandroasilva/gothinkdb/releases/download/v$LatestVersion/$BinaryName"
$TempFile = "$env:TEMP\$BinaryName"

Write-Info "Downloading $BinaryName..."
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempFile -UseBasicParsing
} catch {
    Write-Error-Custom "Failed to download binary"
    exit 1
}

# Install
$InstallPath = "$InstallDir\gothinkdb.exe"
Move-Item -Path $TempFile -Destination $InstallPath -Force

Write-Success "Installed to $InstallPath"
Write-Host ""

# Add to PATH if not already there
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($CurrentPath -notlike "*$InstallDir*") {
    Write-Info "Adding to PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Success "Added to user PATH"
} else {
    Write-Success "Already in PATH"
}

Write-Host ""

# Create data directory
$DataDir = "$env:USERPROFILE\.gothinkdb"
if (!(Test-Path $DataDir)) {
    New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
}
Write-Success "Created data directory: $DataDir"
Write-Host ""

# Verify installation
try {
    $Version = & gothinkdb --version 2>&1
    Write-Success "GoThinkDB is ready!"
} catch {
    Write-Error-Custom "Installation failed - gothinkdb not found in PATH"
    Write-Host "Please restart your terminal and try again"
    exit 1
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Blue
Write-Host "Installation Complete!" -ForegroundColor Blue
Write-Host "==========================================" -ForegroundColor Blue
Write-Host ""
Write-Host "Start GoThinkDB:"
Write-Host "  gothinkdb -data $DataDir"
Write-Host ""
Write-Host "Or run as Windows service (future):"
Write-Host "  gothinkdb -data $DataDir --service"
Write-Host ""
Write-Host "Access dashboard:"
Write-Host "  http://localhost:8080"
Write-Host ""
Write-Host "Default credentials:"
Write-Host "  Username: admin"
Write-Host "  Password: admin"
Write-Host ""
Write-Host "Stop with: Ctrl+C or close the window"
Write-Host ""
