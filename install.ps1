# Windows PowerShell installation script for komari-slim Agent

# Logging functions with colors
function Log-Info { param([string]$Message) Write-Host "$Message"    -ForegroundColor Cyan }
function Log-Success { param([string]$Message) Write-Host "$Message"    -ForegroundColor Green }
function Log-Warning { param([string]$Message) Write-Host "[WARNING] $Message"    -ForegroundColor Yellow }
function Log-Error { param([string]$Message) Write-Host "[ERROR] $Message"    -ForegroundColor Red }
function Log-Step { param([string]$Message) Write-Host "$Message"    -ForegroundColor Magenta }
function Log-Config { param([string]$Message) Write-Host "- $Message"    -ForegroundColor White }

# Default parameters
$InstallDir = Join-Path $Env:ProgramFiles "Komari"
$ServiceName = "komari-agent"
$GitHubProxy = ""
$KomariArgs = @()
$InstallVersion = ""
$InstallSkipChecksum = $false

# Parse script arguments
for ($i = 0; $i -lt $args.Count; $i++) {
    switch ($args[$i]) {
        "--install-dir" { $InstallDir = $args[$i + 1]; $i++; continue }
        "--install-service-name" { $ServiceName = $args[$i + 1]; $i++; continue }
        "--install-ghproxy" { $GitHubProxy = $args[$i + 1]; $i++; continue }
        "--install-version" { $InstallVersion = $args[$i + 1]; $i++; continue }
        "--install-skip-checksum" { $InstallSkipChecksum = $true; continue }
        Default { $KomariArgs += $args[$i] }
    }
}

# Ensure running as Administrator
if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    ).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
    Log-Error "Please run this script as Administrator."
    exit 1
}

# Prepare GitHub proxy display
if ($GitHubProxy -ne '') { $ProxyDisplay = $GitHubProxy } else { $ProxyDisplay = '(direct)' }

# Detect architecture early for constructing binary name
switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    'x86' { $arch = '386' }
    Default { Log-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"; exit 1 }
}

# Ensure installation directory exists for nssm and agent
Log-Step "Ensuring installation directory exists: $InstallDir"
New-Item -ItemType Directory -Path $InstallDir -Force -ErrorAction SilentlyContinue | Out-Null # Ensure $InstallDir exists

# Check for nssm and download if not present
$nssmExeToUse = Join-Path $InstallDir "nssm.exe"

# First, check if nssm is in PATH and is functional
$nssmCmd = Get-Command nssm -ErrorAction SilentlyContinue
if ($nssmCmd) {
    Log-Info "nssm found in PATH at $($nssmCmd.Source)."
    try {
        $nssmVersionOutput = nssm version 2>&1
        Log-Info "Detected nssm version: $nssmVersionOutput"
    }
    catch {
        Log-Warning "nssm found in PATH failed to execute 'nssm version'. Will attempt to use/download local copy. Error: $_"
        $nssmCmd = $null # Force re-evaluation for local copy or download
    }
}

# If nssm not found in PATH or the one in PATH failed, check local $InstallDir
if (-not $nssmCmd) {
    if (Test-Path $nssmExeToUse) {
        Log-Info "nssm found at $nssmExeToUse. Attempting to use it by adding $InstallDir to PATH."
        $env:Path = "$($InstallDir);$($env:Path)"
        $nssmCmd = Get-Command nssm -ErrorAction SilentlyContinue
        if ($nssmCmd) {
            try {
                $nssmVersionOutput = nssm version 2>&1
            }
            catch {
                Log-Warning "nssm from $InstallDir failed to execute 'nssm version'. Error: $_"
                $nssmCmd = $null # Mark as unusable
            }
        }
        else {
            Log-Warning "Failed to make nssm from $nssmExeToUse available via PATH. Will attempt download."
        }
    }
}

# If still no usable nssm command, proceed to download
if (-not $nssmCmd) {
    Log-Info "nssm not found or not usable. Attempting to download to $InstallDir..."
    $NssmVersion = "2.24"
    $NssmZipUrl = "https://nssm.cc/release/nssm-$NssmVersion.zip"
    $TempNssmZipPath = Join-Path $env:TEMP "nssm-$NssmVersion.zip"
    $TempExtractDir = Join-Path $env:TEMP "nssm_extract_temp"

    try {
        Log-Info "Downloading nssm from $NssmZipUrl..."
        Invoke-WebRequest -Uri $NssmZipUrl -OutFile $TempNssmZipPath -UseBasicParsing

        if (Test-Path $TempExtractDir) { Remove-Item -Recurse -Force $TempExtractDir }
        New-Item -ItemType Directory -Path $TempExtractDir -Force | Out-Null
        Expand-Archive -Path $TempNssmZipPath -DestinationPath $TempExtractDir -Force
        
        $NssmSourceDirInsideZip = "nssm-$NssmVersion" # Used for Get-ChildItem search path
        # The path part within the extracted nssm folder, e.g., "nssm-2.24\win32"
        # 'win32' nssm is used for both 'amd64' and 'arm64' PowerShell architectures.
        $NssmArchSubDir = Join-Path "nssm-$NssmVersion" "win32"
        $NssmSourceExePath = Join-Path (Join-Path $TempExtractDir $NssmArchSubDir) "nssm.exe"

        if (-not (Test-Path $NssmSourceExePath)) {
            Log-Error "Could not find nssm.exe at expected path: $NssmSourceExePath after extraction."
            # Fallback search for nssm.exe within the extracted directory
            $foundNssmFallback = Get-ChildItem -Path $TempExtractDir -Recurse -Filter "nssm.exe" | 
            Where-Object { $_.FullName -like "*$NssmArchSubDir\nssm.exe" } | 
            Select-Object -First 1
            if ($foundNssmFallback) {
                Log-Warning "Found nssm.exe at $($foundNssmFallback.FullName) using fallback search. Using this."
                $NssmSourceExePath = $foundNssmFallback.FullName
            }
            else {
                Log-Error "nssm.exe ($NssmArchSubDir) still not found in $TempExtractDir. Please install nssm manually (from https://nssm.cc) and ensure it's in your PATH."
                exit 1
            }
        }
        
        Copy-Item -Path $NssmSourceExePath -Destination $nssmExeToUse -Force

        $env:Path = "$($InstallDir);$($env:Path)"
        $nssmCmd = Get-Command nssm -ErrorAction SilentlyContinue # Re-check after adding to PATH
        if ($nssmCmd) {
            Log-Success "Downloaded nssm is now configured and available in PATH."
        }
        else {
            Log-Error "Failed to configure downloaded nssm in PATH from $nssmExeToUse. Please ensure $InstallDir is in your system PATH or nssm is installed globally."
            exit 1
        }
    }
    catch {
        Log-Error "Failed to download or configure nssm: $_"
        Log-Error "Please install nssm manually from https://nssm.cc and ensure nssm.exe is in your PATH."
        exit 1
    }
    finally {
        if (Test-Path $TempNssmZipPath) { Remove-Item $TempNssmZipPath -Force -ErrorAction SilentlyContinue }
        if (Test-Path $TempExtractDir) { Remove-Item $TempExtractDir -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

# Final check that nssm is operational
try {
    $nssmVersionOutput = nssm version 2>&1
}
catch {
    Log-Error "nssm command failed to execute even after setup attempts. Please check the nssm installation and PATH. Error: $_"
    exit 1
}

Log-Step "Installation configuration:"
Log-Config "Service name: $ServiceName"
Log-Config "Install directory: $InstallDir"
Log-Config "GitHub proxy: $ProxyDisplay"
Log-Config "Agent arguments: $($KomariArgs -join ' ')"
if ($InstallVersion -ne "") {
    Log-Config "Specified agent version: $InstallVersion"
} else {
    Log-Config "Agent version: Latest"
}

# Paths
$BinaryName = "komari-agent-windows-$arch.exe"
$AgentPath = Join-Path $InstallDir "komari-agent.exe"

# Uninstall previous service and binary
function Uninstall-Previous {
    Log-Step "Checking for existing service..."
    # Check if service exists using nssm status, as Get-Service might not work for nssm services if not properly registered
    $serviceStatus = nssm status $ServiceName 2>&1
    if ($serviceStatus -notmatch "SERVICE_STOPPED" -and $serviceStatus -notmatch "does not exist") {
        Log-Info "Stopping service $ServiceName..."
        nssm stop $ServiceName 2>&1 | Out-Null
    }
    # Attempt to remove the service using nssm
    # We check if it exists first by trying to get its status.
    # nssm remove will succeed if the service exists, and fail otherwise.
    # We add confirm to avoid interactive prompts.
    $removeOutput = nssm remove $ServiceName confirm 2>&1
    if ($LASTEXITCODE -eq 0) {
    }
    elseif ($removeOutput -match "Can't open service! (The specified service does not exist as an installed service.)" -or $removeOutput -match "No such service" -or $removeOutput -match "does not exist") {
        Log-Info "Service $ServiceName does not exist or was already removed."
    }
    else {
        # If nssm remove fails for other reasons, try sc.exe delete as a fallback for older installations
        $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($svc) {
            Stop-Service $ServiceName -Force -ErrorAction SilentlyContinue
            sc.exe delete $ServiceName | Out-Null
        }
    }

    # NOTE: the old binary is deliberately NOT removed here any more.
    #
    # The previous order was "delete the old binary, then download". If every
    # download source failed, the machine was left with no service, no unit and
    # no program - the agent was dead and could not recover on its own.
    # Replacement now happens via "Move-Item -Force" after the download has
    # been verified, so a failed replacement still leaves the old binary intact.
    #
    # Stopping the service must still happen before the replacement, because
    # Windows refuses to overwrite a running executable.
}

# NOTE: the Uninstall-Previous call has moved below, to after the download has
# been verified, matching the order used by install.sh.

function Get-LatestSnapshotVersion {
    param([Parameter(Mandatory = $true)][string]$AssetName)

    $ApiUrl = "https://api.github.com/repos/aomtest/komari-slim-agent/releases?per_page=100"
    $ApiUrls = @($ApiUrl)
    if ($GitHubProxy -ne "") {
        $ApiUrls = @("$GitHubProxy/$ApiUrl", $ApiUrl)
    }

    for ($i = 0; $i -lt $ApiUrls.Count; $i++) {
        try {
            Log-Info "Fetching snapshot releases from GitHub API..."
            $releases = Invoke-RestMethod -Uri $ApiUrls[$i] -UseBasicParsing
        }
        catch {
            $releases = $null
        }

        if ($releases) {
            $latestSnapshot = $releases |
            Where-Object {
                $_.draft -eq $false -and
                $_.prerelease -eq $true -and
                $_.tag_name -like "Snapshot-*" -and
                (@($_.assets.name) -contains $AssetName)
            } |
            Sort-Object -Property @{ Expression = { [datetime]$_.published_at }; Descending = $true }, @{ Expression = { $_.tag_name }; Descending = $true } |
            Select-Object -First 1

            if ($latestSnapshot) {
                return $latestSnapshot.tag_name
            }
        }

        if ($i -lt ($ApiUrls.Count - 1)) {
            Log-Warning "Failed to resolve snapshot releases through GitHub proxy, retrying directly."
        }
    }

    throw "No snapshot release contains asset $AssetName."
}

$versionToInstall = ""
if ($InstallVersion -ne "") {
    Log-Info "Attempting to install specified version: $InstallVersion"
    if ($InstallVersion -ieq "snapshot") {
        Log-Info "Resolving the latest snapshot version..."
        try {
            $versionToInstall = Get-LatestSnapshotVersion -AssetName $BinaryName
            Log-Success "Latest snapshot version fetched: $versionToInstall"
        }
        catch {
            Log-Error "Failed to resolve the latest snapshot version: $_"
            exit 1
        }
    }
    else {
        $versionToInstall = $InstallVersion
    }
}
else {
    $ApiUrl = "https://api.github.com/repos/aomtest/komari-slim-agent/releases/latest"
    try {
        Log-Step "Fetching latest release version from GitHub API..."
        $release = Invoke-RestMethod -Uri $ApiUrl -UseBasicParsing
        $versionToInstall = $release.tag_name
        Log-Success "Latest version fetched: $versionToInstall"
    }
    catch {
        Log-Error "Failed to fetch latest version: $_"
        exit 1
    }
}
Log-Success "Installing komari-slim Agent version: $versionToInstall"

# Construct download URL
$BinaryName = "komari-agent-windows-$arch.exe"
$ReleaseBase = "https://github.com/aomtest/komari-slim-agent/releases/download/$versionToInstall"
$DownloadUrl = if ($GitHubProxy) { "$GitHubProxy/$ReleaseBase/$BinaryName" } else { "$ReleaseBase/$BinaryName" }

# Download sources: try mirrors in order when the direct URL fails, matching
# install.sh. SHA256SUMS deliberately does NOT go through these - see the
# verification section below.
$DownloadUrls = @($DownloadUrl)
if (-not $GitHubProxy) {
    $DownloadUrls += @(
        "https://ghfast.top/$DownloadUrl",
        "https://gh-proxy.com/$DownloadUrl",
        "https://ghproxy.net/$DownloadUrl"
    )
}

New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

# Download to a temp file first. Writing straight to $AgentPath would leave a
# half-written exe in place if the download were interrupted.
$TempPath = "$AgentPath.new"
if (Test-Path $TempPath) { Remove-Item $TempPath -Force }

$downloaded = $false
foreach ($url in $DownloadUrls) {
    Log-Step "Downloading $BinaryName ..."
    Log-Info "URL: $url"
    try {
        Invoke-WebRequest -Uri $url -OutFile $TempPath -UseBasicParsing
        if ((Test-Path $TempPath) -and (Get-Item $TempPath).Length -gt 0) {
            $downloaded = $true
            break
        }
    }
    catch {
        Log-Warning "Download failed from this source: $_"
    }
    if (Test-Path $TempPath) { Remove-Item $TempPath -Force }
}

if (-not $downloaded) {
    Log-Error "Download failed from all sources (direct + mirrors)."
    Log-Error "Retry later, or specify --install-ghproxy <mirror-prefix> manually."
    exit 1
}

# Verification. SHA256SUMS is always fetched from GitHub directly, never
# through a mirror: a mirror exists to speed up downloads, not to vouch for
# origin. If both the binary and the checksum came from the same mirror, a
# compromised mirror could forge the pair and the comparison would prove
# nothing.
Log-Step "Verifying checksum..."
$SumsUrl = "$ReleaseBase/SHA256SUMS"
$expected = $null
try {
    $sumsContent = (Invoke-WebRequest -Uri $SumsUrl -UseBasicParsing).Content
    if ($sumsContent -is [byte[]]) { $sumsContent = [Text.Encoding]::UTF8.GetString($sumsContent) }
    foreach ($line in ($sumsContent -split "`n")) {
        $parts = @($line.Trim() -split '\s+')
        if ($parts.Count -ge 2 -and $parts[1] -eq $BinaryName) { $expected = $parts[0]; break }
    }
}
catch {
    $expected = $null
}

# 0 = verified, 1 = checksum mismatch, 2 = could not verify
if (-not $expected) {
    $verifyStatus = 2
}
else {
    try {
        $actual = (Get-FileHash -Path $TempPath -Algorithm SHA256).Hash
    }
    catch {
        $actual = $null
    }
    if (-not $actual) { $verifyStatus = 2 }
    elseif ($actual.ToLower() -ne $expected.ToLower()) { $verifyStatus = 1 }
    else { $verifyStatus = 0 }
}

switch ($verifyStatus) {
    0 {
        Log-Success "Checksum verified against the official SHA256SUMS."
    }
    1 {
        Log-Error "Checksum mismatch for $BinaryName."
        Log-Error "The downloaded file does not match the official SHA256SUMS - it was"
        Log-Error "tampered with or corrupted. Installation aborted; nothing was changed."
        Remove-Item $TempPath -Force
        exit 1
    }
    default {
        if ($InstallSkipChecksum) {
            Log-Warning "Could not verify $BinaryName, continuing because --install-skip-checksum was given."
            Log-Warning "The integrity of this binary has NOT been confirmed."
        }
        else {
            Log-Error "Could not verify $BinaryName against the official SHA256SUMS."
            Log-Error "  - SHA256SUMS was unreachable (GitHub may be blocked), or"
            Log-Error "  - $BinaryName is not listed in it."
            Log-Error "Nothing has been changed on this system."
            Log-Error "Re-run with --install-skip-checksum to install without verification (not recommended)."
            Remove-Item $TempPath -Force
            exit 1
        }
    }
}

# Only touch system state after the download has been verified: stop the
# service and remove the old service registration. This has to happen before
# the replacement, because Windows refuses to overwrite a running executable.
Uninstall-Previous

Move-Item -Path $TempPath -Destination $AgentPath -Force
Log-Success "Installed to $AgentPath"

# Register and start service
Log-Step "Configuring Windows service with nssm..."
$argString = $KomariArgs -join ' '
# Ensure InstallDir and AgentPath are quoted if they contain spaces
$quotedAgentPath = "`"$AgentPath`""
nssm install $ServiceName $quotedAgentPath $argString
# Set display name and startup type using nssm
nssm set $ServiceName DisplayName "komari-slim Agent Service"
nssm set $ServiceName Start SERVICE_AUTO_START
nssm set $ServiceName AppExit Default Restart
nssm set $ServiceName AppRestartDelay 5000
# Start the service using nssm
nssm start $ServiceName
Log-Success "Service $ServiceName installed and started using nssm."

Log-Success "komari-slim Agent installation completed!"
Log-Config "Service name: $ServiceName"
Log-Config "Arguments: $argString"
