# Windows PowerShell installation script for Komari Agent

# Logging functions with colors
function Log-Info   { Write-Host "[INFO]    $($_)"    -ForegroundColor Cyan }
function Log-Success{ Write-Host "[SUCCESS] $($_)"    -ForegroundColor Green }
function Log-Warning{ Write-Host "[WARNING] $($_)"    -ForegroundColor Yellow }
function Log-Error  { Write-Host "[ERROR]   $($_)"    -ForegroundColor Red }
function Log-Step   { Write-Host "[STEP]    $($_)"    -ForegroundColor Magenta }
function Log-Config { Write-Host "[CONFIG]  $($_)"    -ForegroundColor White }

# Default parameters
$InstallDir      = Join-Path $Env:ProgramFiles "Komari"
$ServiceName     = "komari-agent"
$GitHubProxy     = ""
$KomariArgs      = @()

# Parse script arguments
for ($i = 0; $i -lt $args.Count; $i++) {
    switch ($args[$i]) {
        "--install-dir"          { $InstallDir = $args[$i+1]; $i++; continue }
        "--install-service-name" { $ServiceName = $args[$i+1]; $i++; continue }
        "--install-ghproxy"      { $GitHubProxy = $args[$i+1]; $i++; continue }
        Default                    { $KomariArgs += $args[$i] }
    }
}

# Ensure running as Administrator
if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    ).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
    Log-Error "Please run this script as Administrator."
    exit 1
}

Log-Step "Installation configuration:"
Log-Config "Service name: $ServiceName"
Log-Config "Install directory: $InstallDir"
Log-Config "GitHub proxy: $($GitHubProxy -ne '' ? $GitHubProxy : '(direct)')"
Log-Config "Agent arguments: $($KomariArgs -join ' ')"

# Paths
$BinaryName   = "komari-agent-windows-$((switch($env:PROCESSOR_ARCHITECTURE) { 'AMD64' { 'amd64' }; 'ARM64' { 'arm64' }; Default { '' } })) .exe".Trim()
$AgentPath    = Join-Path $InstallDir $BinaryName

# Uninstall previous service and binary
function Uninstall-Previous {
    Log-Step "Checking for existing service..."
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($svc) {
        Log-Info "Stopping service $ServiceName..."
        Stop-Service $ServiceName -Force
        Log-Info "Deleting service $ServiceName..."
        sc.exe delete $ServiceName | Out-Null
    }
    if (Test-Path $AgentPath) {
        Log-Info "Removing old binary..."
        Remove-Item $AgentPath -Force
    }
}
Uninstall-Previous

# Detect architecture
switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $arch = "amd64" }
    "ARM64" { $arch = "arm64" }
    Default  { Log-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"; exit 1 }
}
Log-Info "Detected architecture: $arch"

# Fetch latest release version
$ApiUrl = "https://api.github.com/repos/komari-monitor/komari-agent/releases/latest"
Log-Step "Fetching latest version from GitHub API..."
try {
    $release        = Invoke-RestMethod -Uri $ApiUrl -UseBasicParsing
    $latestVersion  = $release.tag_name
} catch {
    Log-Error "Failed to fetch latest version: $_"
    exit 1
}
Log-Success "Latest version: $latestVersion"

# Construct download URL
$BinaryName  = "komari-agent-windows-$arch.exe"
$DownloadUrl = if ($GitHubProxy) { "$GitHubProxy/https://github.com/komari-monitor/komari-agent/releases/download/$latestVersion/$BinaryName" } else { "https://github.com/komari-monitor/komari-agent/releases/download/$latestVersion/$BinaryName" }

# Download and install
Log-Step "Preparing installation directory..."
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Log-Step "Downloading $BinaryName..."
Log-Info "URL: $DownloadUrl"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $AgentPath -UseBasicParsing
} catch {
    Log-Error "Download failed: $_"
    exit 1
}
Log-Success "Downloaded and saved to $AgentPath"

# Register and start service
Log-Step "Configuring Windows service..."
$argString = $KomariArgs -join ' '
New-Service -Name $ServiceName -BinaryPathName "`"$AgentPath`" $argString" -DisplayName "Komari Agent Service" -StartupType Automatic
Start-Service $ServiceName
Log-Success "Service $ServiceName installed and started."

Log-Success "Komari Agent installation completed!"
Log-Config "Service name: $ServiceName"
Log-Config "Arguments: $argString"
