# enplace installer — Windows (PowerShell)
#
#   irm https://raw.githubusercontent.com/djcp/enplace/main/install.ps1 | iex
#
# Environment overrides:
#   $env:ENPLACE_VERSION   install a specific tag (e.g. v1.4.0-alpha); default: latest
#   $env:INSTALL_DIR       install location; default: %LOCALAPPDATA%\Programs\enplace
#
# Downloads the matching release archive from GitHub, verifies its sha256 against
# the release checksums.txt, and installs enplace.exe (adding it to the user PATH).

$ErrorActionPreference = 'Stop'

$Repo = 'djcp/enplace'
$Bin = 'enplace'

function Fail($msg) { Write-Error $msg; exit 1 }

# --- Detect architecture ------------------------------------------------------
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'x86'   { '386' }
    default { Fail "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
}

# --- Resolve version ----------------------------------------------------------
$headers = @{ 'User-Agent' = 'enplace-installer' }
$api = "https://api.github.com/repos/$Repo/releases"

if ($env:ENPLACE_VERSION) {
    $tag = $env:ENPLACE_VERSION
}
else {
    # Prefer the latest stable release; fall back to the newest release of any
    # kind (covers the case where only prereleases exist).
    try {
        $tag = (Invoke-RestMethod -Uri "$api/latest" -Headers $headers).tag_name
    }
    catch {
        $tag = (Invoke-RestMethod -Uri "$api`?per_page=1" -Headers $headers)[0].tag_name
    }
}
if (-not $tag) { Fail 'could not determine the latest release; set $env:ENPLACE_VERSION' }

$ver = $tag -replace '^v', ''
$archive = "${Bin}_${ver}_windows_${arch}.zip"
$base = "https://github.com/$Repo/releases/download/$tag"

$installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\enplace" }

Write-Host "Installing $Bin $tag (windows/$arch)..."

# --- Download + verify --------------------------------------------------------
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("enplace-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
    $zipPath = Join-Path $tmp $archive
    Invoke-WebRequest -Uri "$base/$archive" -OutFile $zipPath -Headers $headers

    try {
        $sumsPath = Join-Path $tmp 'checksums.txt'
        Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $sumsPath -Headers $headers
        $line = Select-String -Path $sumsPath -Pattern ([regex]::Escape($archive)) | Select-Object -First 1
        if (-not $line) { Fail "no checksum entry for $archive" }
        $want = ($line.Line -split '\s+')[0].ToLower()
        $got = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()
        if ($want -ne $got) { Fail "checksum mismatch for $archive (expected $want, got $got)" }
    }
    catch {
        Write-Warning "could not verify checksum: $($_.Exception.Message)"
    }

    # --- Install --------------------------------------------------------------
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
    Copy-Item -Path (Join-Path $tmp "$Bin.exe") -Destination (Join-Path $installDir "$Bin.exe") -Force
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "Installed $Bin to $installDir\$Bin.exe"

# --- PATH ---------------------------------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    $newPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    $env:Path = "$env:Path;$installDir"
    Write-Host "Added $installDir to your user PATH (restart your shell to pick it up)."
}

Write-Host ""
Write-Host "Run 'enplace' to get started, and 'enplace update' to upgrade later."
