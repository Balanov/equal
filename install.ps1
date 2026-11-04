# Install EQUAL from the latest GitHub release with a SHA-256 checksum check.
# Usage: irm https://raw.githubusercontent.com/Balanov/equal/main/install.ps1 | iex
# Optional: $env:EQUAL_VERSION = "0.1.0"; $env:EQUAL_INSTALL_DIR = "C:\Tools"
$ErrorActionPreference = "Stop"
$repo = "Balanov/equal"
$version = $env:EQUAL_VERSION
if (-not $version) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ "User-Agent" = "equal-installer" }
    $version = $release.tag_name
}
$version = $version.TrimStart("v")
if (-not $version) { throw "equal: could not resolve release version" }

$archName = ""
try {
    $archName = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
} catch {
    $archName = ($env:PROCESSOR_ARCHITEW6432, $env:PROCESSOR_ARCHITECTURE | Where-Object { $_ } | Select-Object -First 1).ToString().ToLowerInvariant()
}
switch ($archName) {
    "x64" { $arch = "amd64" }
    "amd64" { $arch = "amd64" }
    "arm64" { $arch = "arm64" }
    default { throw "equal: unsupported architecture '$archName'" }
}

$asset = "equal-windows-$arch.exe"
$base = "https://github.com/$repo/releases/download/v$version"
$installDir = $env:EQUAL_INSTALL_DIR
if (-not $installDir) { $installDir = "$env:LOCALAPPDATA\Programs\EQUAL" }
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("equal-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt")
    $line = Get-Content (Join-Path $tmp "checksums.txt") | Where-Object { ($_ -split "\s+", 2)[1].TrimStart("*") -eq $asset } | Select-Object -First 1
    if (-not $line) { throw "equal: no checksum for $asset" }
    $expected = ($line -split "\s+", 2)[0]
    $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $asset)).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "equal: checksum mismatch for $asset" }

    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $target = Join-Path $installDir "equal.exe"
    Copy-Item (Join-Path $tmp $asset) $target -Force
    $installed = $null
    for ($attempt = 1; $attempt -le 5; $attempt++) {
        try {
            $installed = (& $target --version 2>$null)
            if ($LASTEXITCODE -eq 0 -and $installed) { break }
        } catch {
            Start-Sleep -Milliseconds 700
        }
    }
    if (-not $installed) { throw "equal: installed binary did not run; check antivirus or file locks" }
    Write-Output "installed: $installed -> $target"
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $installDir) {
        Write-Output "note: $installDir is not in PATH"
    }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
