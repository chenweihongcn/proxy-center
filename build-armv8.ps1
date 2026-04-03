# Build proxy-center for ARM64 (armv8) architecture - Windows PowerShell
# Usage: .\build-armv8.ps1 -Version "1.0.0" -Push $true

param(
    [string]$Version = "dev-$(Get-Date -Format yyyyMMddHHmmss)",
    [string]$ImageName = "proxy-center",
    [bool]$Push = $false,
    [string]$Registry = "docker.io",
    [string]$iStoreOSIP = ""
)

$ErrorActionPreference = "Stop"
$ImageTag = "armv8-${Version}"
$FullImage = "${Registry}/${ImageName}:${ImageTag}"

Write-Host "📦 Building ${ImageName}:${ImageTag} for linux/arm64" -ForegroundColor Cyan

# Method 1: Docker buildx (if available)
$buildxAvailable = $false
try {
    $null = docker buildx ls 2>$null
    $buildxAvailable = $true
} catch {
    $buildxAvailable = $false
}

if ($buildxAvailable) {
    Write-Host "✓ Using docker buildx for multi-platform build" -ForegroundColor Green
    
    $args = @(
        "buildx", "build",
        "--platform", "linux/arm64",
        "--tag", "${ImageName}:${ImageTag}",
        "--tag", "${ImageName}:latest-armv8"
    )
    
    if ($Push) {
        $args += "--push"
    } else {
        $args += "--output", "type=docker"
    }
    
    $args += "."
    
    & docker $args
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Build successful: ${ImageName}:${ImageTag}" -ForegroundColor Green
        exit 0
    }
}

# Method 2: Compile on remote iStoreOS via SSH
if ($iStoreOSIP) {
    Write-Host "✓ Compiling on remote iStoreOS: $iStoreOSIP" -ForegroundColor Green
    
    # Check SSH connectivity
    try {
        $null = ssh -o ConnectTimeout=3 "root@$iStoreOSIP" "echo ok" 2>$null
    } catch {
        Write-Host "❌ Cannot connect to iStoreOS at $iStoreOSIP via SSH" -ForegroundColor Red
        exit 1
    }
    
    # Transfer source
    Write-Host "📤 Transferring source code..." -ForegroundColor Gray
    scp -O -r "cmd" "internal" "go.mod" "go.sum" "root@${iStoreOSIP}:/tmp/proxy-center/" 2>$null
    
    # Remote build
    Write-Host "⚙️  Building on remote system..." -ForegroundColor Gray
    ssh "root@$iStoreOSIP" @"
        cd /tmp/proxy-center
        export CGO_ENABLED=0
        export GOOS=linux
        export GOARCH=arm64
        go build -ldflags="-s -w -X main.version=${Version}" -o /tmp/proxyd ./cmd/proxyd
"@
    
    # Retrieve binary
    Write-Host "📥 Retrieving compiled binary..." -ForegroundColor Gray
    if (-not (Test-Path "build")) { mkdir "build" | Out-Null }
    scp -O "root@${iStoreOSIP}:/tmp/proxyd" "build/proxyd-arm64"
    
    Write-Host "✓ Binary saved to build/proxyd-arm64" -ForegroundColor Green
    exit 0
}

Write-Host "ℹ️  Build methods available:" -ForegroundColor Yellow
Write-Host "  1. docker buildx (if installed): automatically detected" -ForegroundColor Gray
Write-Host "  2. Remote iStoreOS compilation: .\build-armv8.ps1 -iStoreOSIP 192.168.50.94" -ForegroundColor Gray
Write-Host ""
Write-Host "❌ No suitable build method available. Install one of:" -ForegroundColor Red
Write-Host "  • docker buildx: https://docs.docker.com/build/install-buildx/" -ForegroundColor Gray
Write-Host "  • OpenSSH client: ssh -V" -ForegroundColor Gray

exit 1
