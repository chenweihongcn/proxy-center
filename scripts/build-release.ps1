# PowerShell script to build release artifacts

param(
    [string]$Version = "dev",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$ReleaseDir = "./release"
$ArtifactDir = "$ReleaseDir/proxy-center-$Version"

Write-Host "📦 Building release artifacts for version: $Version" -ForegroundColor Cyan
Write-Host ""

# Clean up old release
if (Test-Path $ReleaseDir) {
    Remove-Item $ReleaseDir -Recurse -Force
}
New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null

Write-Host "[1/6] Copying source files..." -ForegroundColor Yellow
New-Item -ItemType Directory -Path "$ArtifactDir/src" -Force | Out-Null
Copy-Item cmd, internal, go.mod, go.sum -Destination "$ArtifactDir/src" -Recurse -Force
Copy-Item Dockerfile -Destination "$ArtifactDir/src" -Force -ErrorAction SilentlyContinue

Write-Host "[2/6] Copying deployment files..." -ForegroundColor Yellow
New-Item -ItemType Directory -Path "$ArtifactDir/deploy" -Force | Out-Null
Get-ChildItem deploy | Copy-Item -Destination "$ArtifactDir/deploy" -Recurse -Force

Write-Host "[3/6] Copying documentation..." -ForegroundColor Yellow
@("README.md", "QUICK_REFERENCE.md", "DELIVERY_CHECKLIST.md", "CHANGELOG.md") | ForEach-Object {
    Copy-Item $_ -Destination "$ArtifactDir" -Force -ErrorAction SilentlyContinue
}
Copy-Item "deploy/ISTOREIOS_DEPLOYMENT.md" -Destination "$ArtifactDir" -Force -ErrorAction SilentlyContinue

Write-Host "[4/6] Creating cross-compilation scripts..." -ForegroundColor Yellow
New-Item -ItemType Directory -Path "$ArtifactDir/scripts" -Force | Out-Null
@("build-armv8.sh", "build-armv8.ps1") | ForEach-Object {
    Copy-Item $_ -Destination "$ArtifactDir/scripts" -Force -ErrorAction SilentlyContinue
}

Write-Host "[5/6] Creating release packages..." -ForegroundColor Yellow

# Create ZIP archive
$ZipFile = "$ReleaseDir/proxy-center-$Version-source.zip"
$CompSource = @{
    Path = $ArtifactDir
    DestinationPath = $ZipFile
    CompressionLevel = "Optimal"
    Force = $true
}
Compress-Archive @CompSource
Write-Host "   ✓ Created: proxy-center-$Version-source.zip" -ForegroundColor Green

# Create TAR.GZ if available
if (Get-Command tar -ErrorAction SilentlyContinue) {
    Push-Location $ReleaseDir
    tar czf "proxy-center-$Version-source.tar.gz" "proxy-center-$Version/"
    Write-Host "   ✓ Created: proxy-center-$Version-source.tar.gz" -ForegroundColor Green
    Pop-Location
}

Write-Host "[6/6] Creating checksums..." -ForegroundColor Yellow

# Generate SHA256 checksums
$ChecksumFile = "$ReleaseDir/SHA256SUMS"
$Checksums = Get-ChildItem $ReleaseDir -Filter "*.zip", "*.tar.gz" | 
    ForEach-Object {
        $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash
        "$hash  $($_.Name)"
    }
$Checksums | Out-File $ChecksumFile -Encoding UTF8

Write-Host ""
Write-Host "✅ Release build completed!" -ForegroundColor Green
Write-Host ""
Write-Host "📁 Artifacts location: $ReleaseDir/" -ForegroundColor Cyan
Get-ChildItem $ReleaseDir -File | Format-Table Name, Length -AutoSize
Write-Host ""
Write-Host "📝 Next steps:" -ForegroundColor Yellow
Write-Host "   1. Review the contents in: $ArtifactDir/"
Write-Host "   2. Upload to GitHub Releases"
Write-Host "   3. Tag the release: git tag -a v$Version -m 'Release v$Version'"
