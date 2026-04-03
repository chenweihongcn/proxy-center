#!/bin/bash
# Integration tests for proxy-center

set -e

echo "🧪 proxy-center Integration Tests"
echo "================================="
echo ""

# Test 1: HTTP CONNECT
echo "[Test 1/5] Testing HTTP CONNECT proxy..."
TEST_URL="http://httpbin.org/get"
if curl -s -x http://admin:change-me-now@localhost:8080 "$TEST_URL" | grep -q "url"; then
    echo "✓ HTTP CONNECT works"
else
    echo "✗ HTTP CONNECT failed"
    exit 1
fi
echo ""

# Test 2: SOCKS5
echo "[Test 2/5] Testing SOCKS5 proxy..."
if curl -s -x socks5://admin:change-me-now@localhost:1080 "$TEST_URL" | grep -q "url"; then
    echo "✓ SOCKS5 works"
else
    echo "✗ SOCKS5 failed"
    exit 1
fi
echo ""

# Test 3: Web API
echo "[Test 3/5] Testing Web API..."
API_RESPONSE=$(curl -s -u admin:change-me-now http://localhost:8090/api/users)
if echo "$API_RESPONSE" | grep -q "admin"; then
    echo "✓ Web API users endpoint works"
else
    echo "✗ Web API failed"
    exit 1
fi
echo ""

# Test 4: Database queries
echo "[Test 4/5] Testing database integrity..."
if docker exec proxy-center sqlite3 /data/proxy-center.db "SELECT COUNT(*) FROM users;" 2>/dev/null | grep -q "[0-9]"; then
    echo "✓ Database query successful"
else
    echo "✗ Database query failed"
    exit 1
fi
echo ""

# Test 5: Health check
echo "[Test 5/5] Testing health check..."
if curl -s http://localhost:8090/healthz | grep -q "ok"; then
    echo "✓ Health check passed"
else
    echo "✗ Health check failed"
    exit 1
fi
echo ""

echo "================================="
echo "✅ All integration tests passed!"
echo ""
echo "Test environment:"
echo "  HTTP Proxy:   http://localhost:8080"
echo "  SOCKS5:       socks5://localhost:1080"
echo "  Web UI:       http://localhost:8090"
echo "  Default user: admin"
