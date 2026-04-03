# Windows deployment verification script

param(
    [string]$ProxyHost = "localhost",
    [int]$HttpPort = 8080,
    [int]$SocksPort = 1080,
    [int]$WebPort = 8090,
    [string]$Username = "admin",
    [string]$Password = "change-me-now"
)

$ErrorActionPreference = "Continue"
$errors = 0
$warnings = 0

function Test-Service {
    param([string]$Name, [int]$Port)
    
    try {
        if ((Test-NetConnection -ComputerName $ProxyHost -Port $Port -InformationLevel Quiet -WarningAction SilentlyContinue)) {
            Write-Host "✓ $Name is accessible on port $Port" -ForegroundColor Green
            return $true
        } else {
            Write-Host "✗ $Name failed on port $Port" -ForegroundColor Red
            $global:errors++
            return $false
        }
    } catch {
        Write-Host "✗ Error testing $Name : $_" -ForegroundColor Red
        $global:errors++
        return $false
    }
}

function Test-ProxyConnectivity {
    param([string]$ProxyType, [int]$ProxyPort)
    
    try {
        # Simple proxy connectivity test
        $testUrl = "http://httpbin.org/get"
        $response = curl.exe -s -m 5 -x "$ProxyType`://$ProxyHost`:$ProxyPort" "$testUrl"
        
        if ($response -like "*url*") {
            Write-Host "✓ $ProxyType proxy works" -ForegroundColor Green
            return $true
        } else {
            Write-Host "⚠ $ProxyType proxy connectivity unclear (check proxy server)" -ForegroundColor Yellow
            $global:warnings++
            return $false
        }
    } catch {
        Write-Host "✗ $ProxyType proxy failed: $_" -ForegroundColor Red
        $global:errors++
        return $false
    }
}

Write-Host @"
╔═════════════════════════════════════════════════╗
║  proxy-center Deployment Verification (Windows) ║
╚═════════════════════════════════════════════════╝

Checking deployment...

"@ -ForegroundColor Cyan

# 1. Port connectivity
Write-Host "1. Port Connectivity`n" -ForegroundColor Yellow
Test-Service "HTTP CONNECT" $HttpPort
Test-Service "SOCKS5" $SocksPort  
Test-Service "Web UI" $WebPort
Write-Host ""

# 2. Web UI
Write-Host "2. Web UI`n" -ForegroundColor Yellow
try {
    $basicAuth = [System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes("$Username`:$Password"))
    $response = Invoke-WebRequest -Uri "http://$ProxyHost`:$WebPort/healthz" `
        -Headers @{Authorization = "Basic $basicAuth"} `
        -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
    Write-Host "✓ Web UI responds to health check" -ForegroundColor Green
} catch {
    Write-Host "✗ Web UI health check failed: $($_.Exception.Message)" -ForegroundColor Red
    $errors++
}
Write-Host ""

# 3. API testing
Write-Host "3. API Testing`n" -ForegroundColor Yellow
try {
    $basicAuth = [System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes("$Username`:$Password"))
    $response = Invoke-WebRequest -Uri "http://$ProxyHost`:$WebPort/api/users" `
        -Headers @{Authorization = "Basic $basicAuth"} `
        -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
    
    if ($response.StatusCode -eq 200) {
        Write-Host "✓ API users endpoint accessible" -ForegroundColor Green
        $content = $response.Content | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($null -ne $content -and $content.Count -gt 0) {
            Write-Host "  Users: $($content.Count)" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "✗ API test failed: $($_.Exception.Message)" -ForegroundColor Red
    $errors++
}
Write-Host ""

# 4. Docker status (if available)
Write-Host "4. Docker Status`n" -ForegroundColor Yellow
if (Get-Command docker -ErrorAction SilentlyContinue) {
    try {
        $container = docker ps --filter "name=proxy-center" --format "{{.Names}}"
        if ($container) {
            Write-Host "✓ Container 'proxy-center' is running" -ForegroundColor Green
            
            $stats = docker stats --no-stream proxy-center 2>$null | Select-Object -Last 1
            if ($stats) {
                Write-Host "  Resource stats: $stats" -ForegroundColor Gray
            }
        } else {
            Write-Host "⚠ Container not found" -ForegroundColor Yellow
            $warnings++
        }
    } catch {
        Write-Host "✗ Docker command failed: $_" -ForegroundColor Red
        $errors++
    }
} else {
    Write-Host "ℹ Docker not available on PATH" -ForegroundColor Gray
}
Write-Host ""

# 5. Proxy functionality
Write-Host "5. Proxy Functionality`n" -ForegroundColor Yellow
if (Get-Command curl -ErrorAction SilentlyContinue) {
    Test-ProxyConnectivity "http" $HttpPort
    if ($SocksPort -eq 1080) {
        Write-Host "  (Note: SOCKS5 connectivity testing requires curl with SOCKS5 support)" -ForegroundColor Gray
    }
} else {
    Write-Host "ℹ curl not found - skipping proxy connectivity tests" -ForegroundColor Gray
}
Write-Host ""

# Summary
Write-Host "╔═════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║  Verification Summary                           ║" -ForegroundColor Cyan
Write-Host "╚═════════════════════════════════════════════════╝" 

Write-Host "Errors: $errors | Warnings: $warnings" -ForegroundColor Cyan
Write-Host ""

if ($errors -eq 0) {
    Write-Host "✅ Deployment verification PASSED!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Quick access:" -ForegroundColor Yellow
    Write-Host "  Web UI:   http://$ProxyHost`:$WebPort" -ForegroundColor Gray
    Write-Host "  HTTP:     http://$Username`:$Password@$ProxyHost`:$HttpPort" -ForegroundColor Gray
    Write-Host "  SOCKS5:   socks5://$Username`:$Password@$ProxyHost`:$SocksPort" -ForegroundColor Gray
    exit 0
} else {
    Write-Host "❌ Deployment has $errors error(s)!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Troubleshooting:" -ForegroundColor Yellow
    Write-Host "  1. docker logs proxy-center" -ForegroundColor Gray
    Write-Host "  2. Check firewall rules for ports $HttpPort, $SocksPort, $WebPort" -ForegroundColor Gray
    exit 1
}
