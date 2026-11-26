# Dockship Windows Installer
# Usage: powershell -ExecutionPolicy Bypass -File install.ps1

$ErrorActionPreference = "Stop"

Write-Host "🚀 Building and installing Dockship for Windows..." -ForegroundColor Cyan

$InstallDir = "$HOME\.dockship\bin"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$env:CGO_ENABLED = "0"
$OutputBinary = "$InstallDir\dockship.exe"

go build -ldflags="-s -w" -o $OutputBinary ./cmd/dockship

Write-Host "✔ Dockship binary installed at: $OutputBinary" -ForegroundColor Green

# Add to User PATH if not present
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "✔ Added $InstallDir to User PATH." -ForegroundColor Green
    Write-Host "ℹ Please restart your terminal or run: `$env:Path += ';$InstallDir'" -ForegroundColor Yellow
} else {
    Write-Host "✔ $InstallDir is already in your PATH." -ForegroundColor Green
}

Write-Host "`n🎉 Dockship is ready! Test it by running: dockship --help" -ForegroundColor Magenta
